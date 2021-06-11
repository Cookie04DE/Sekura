package main

import (
	"encoding/gob"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	rubberhose "github.com/Cookie04DE/RubberHose"
)

const (
	sockPath string = "/run/sekura.sock"
	pidPath  string = "/run/sekura.pid"
)

var disks = make(map[string]rubberhose.Disk)

func main() {
	if _, err := os.Stat(pidPath); err == nil {
		log.Fatal("Pid file already exists")
	}
	err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0755)
	if err != nil {
		log.Fatal("Error writing pid: " + err.Error())
	}
	defer os.Remove(pidPath)
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		log.Fatal("Error opening socket: " + err.Error())
	}
	sigC := make(chan os.Signal)
	signal.Notify(sigC, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sigC
		ln.Close()
	}()
	rubberhose.RegisterGob()
	for {
		if func() bool {
			conn, err := ln.Accept()
			if errors.Is(err, net.ErrClosed) {
				return true
			}
			if err != nil {
				return false
			}
			go func() {
				defer conn.Close()
				d := gob.NewDecoder(conn)
				e := gob.NewEncoder(conn)
			outer:
				for {
					request := &rubberhose.Request{}
					err := d.Decode(request)
					if err != nil {
						break
					}
					switch request.ID {
					default:
						break outer
					case rubberhose.AddRequestID:
						ar := request.Data.(*rubberhose.AddRequest)
						dp := ar.DiskPath
						disk, ok := disks[dp]
						if !ok {
							d, err := rubberhose.NewDisk(dp)
							if err != nil {
								err := e.Encode(&rubberhose.AddResponse{Error: err.Error()})
								if err != nil {
									break outer
								}
								break
							}
							disk = *d
							disks[dp] = disk
						}
						partition, err := disk.GetPartition(ar.Password)
						if err != nil {
							err := e.Encode(&rubberhose.AddResponse{Error: err.Error()})
							if err != nil {
								break outer
							}
							break
						}
						devicePath, _ := partition.Expose()
						if err != nil {
							err := e.Encode(&rubberhose.AddResponse{Error: err.Error()})
							if err != nil {
								break outer
							}
						}
						err = e.Encode(&rubberhose.AddResponse{DevicePath: devicePath})
						if err != nil {
							break outer
						}
					case rubberhose.DeleteRequestID:
						dr := request.Data.(*rubberhose.DeleteRequest)
						dp := dr.DiskPath
						disk, ok := disks[dp]
						if !ok {
							d, err := rubberhose.NewDisk(dp)
							if err != nil {
								err := e.Encode(&rubberhose.AddResponse{Error: err.Error()})
								if err != nil {
									break outer
								}
								break
							}
							disk = *d
							disks[dp] = disk
						}
						partition, err := disk.GetPartition(dr.Password)
						if err != nil {
							err := e.Encode(&rubberhose.AddResponse{Error: err.Error()})
							if err != nil {
								break outer
							}
							break
						}
						err = partition.Delete()
						errstring := ""
						if err != nil {
							errstring = err.Error()
						}
						err = e.Encode(&rubberhose.DeleteResponse{Error: errstring})
						if err != nil {
							break outer
						}
					}
				}
			}()
			return false
		}() {
			break
		}
	}
}
