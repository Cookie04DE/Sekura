package main

import (
	"bufio"
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	rubberhose "github.com/Cookie04DE/RubberHose"
	"github.com/shenwei356/util/bytesize"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func fatalParsable(parsable bool, a ...interface{}) {
	if !parsable {
		log.Fatal(a...)
	}
	log.Fatal()
}

func getPassword(password *string, parsable bool) string {
	if pw := *password; pw != "" {
		return pw
	}
	if !parsable {
		fmt.Print("Please enter the password: ")
	}
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		fatalParsable(parsable, "Error reading password: ", err)
	}
	pw := string(passwordBytes)
	if pw == "" {
		fatalParsable(parsable, "Empty password!")
	}
	return pw
}

func main() {
	standalone := flag.Bool("standalone", false, "Runs Sekura in standalone mode not relying on the daemon.")
	parsable := flag.Bool("parsable", false, "Provide output in machine parsable output instead of human readable format")
	disk := flag.String("disk", "", "The sekura disk to work on")
	password := flag.String("password", "", "The password of the partition to work on (can also be provided interactively)")
	flag.Parse()
	if *standalone {
		runStandaloneMode()
		return
	}
	if len(os.Args) == 1 {
		usage()
		return
	}
	conn, err := net.Dial("unix", "/run/sekura.sock")
	if err != nil {
		log.Fatal("Error opening connection to daemon: " + err.Error())
	}
	rubberhose.RegisterGob()
	d := gob.NewDecoder(conn)
	e := gob.NewEncoder(conn)
	switch flag.Arg(0) {
	default:
		usage()
		return
	case "add":
		if *disk == "" {
			log.Fatal("Please provide a disk with the -disk flag")
		}
		absPath, err := filepath.Abs(*disk)
		if err != nil {
			log.Fatal("Error turning path into absolute path: " + err.Error())
		}
		pw := getPassword(password, *parsable)
		err = e.Encode(&rubberhose.Request{ID: rubberhose.AddRequestID, Data: rubberhose.AddRequest{DiskPath: absPath, Password: pw}})
		if err != nil {
			log.Fatal("Error writing to daemon socket: " + err.Error())
		}
		response := &rubberhose.AddResponse{}
		err = d.Decode(response)
		if err != nil {
			log.Fatal("Error reading from daemon socket: " + err.Error())
		}
		if response.Error != "" {
			log.Fatal("Deamon reported error while adding partition: " + response.Error)
		}
		if *parsable {
			fmt.Print(response.DevicePath)
			return
		}
		fmt.Println("Success. Device Path: " + response.DevicePath)
	case "delete":
		if *disk == "" {
			log.Fatal("Please provide a disk with the -disk flag")
		}
		absPath, err := filepath.Abs(*disk)
		if err != nil {
			log.Fatal("Error turning path into absolute path: " + err.Error())
		}
		pw := getPassword(password, *parsable)
		err = e.Encode(&rubberhose.Request{ID: rubberhose.DeleteRequestID, Data: rubberhose.DeleteRequest{DiskPath: absPath, Password: pw}})
		if err != nil {
			log.Fatal("Error writing to daemon socket: " + err.Error())
		}
		response := &rubberhose.DeleteResponse{}
		err = d.Decode(response)
		if err != nil {
			log.Fatal("Error reading from daemon socket: " + err.Error())
		}
		if response.Error != "" {
			log.Fatal("Deamon reported error while deleting partition: " + response.Error)
		}
		if *parsable {
			return
		}
		fmt.Println("Successfully deleted partition!")
	}
}

func usage() {
	fmt.Println(`Sekura CLI
Commands:
 add: -disk required, -password optional
 remove: -disk required -password optional
Example:
$ sekura -disk /path/to/my/disk add`)
}

func runStandaloneMode() {
	if unix.Geteuid() != 0 {
		log.Fatal("Sekura requires root permissions to work")
	}
	disks := []*rubberhose.Disk{}
	scanner := bufio.NewScanner(os.Stdin)
scanloop:
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		in := strings.Trim(scanner.Text(), " \t")
		if in == "" {
			continue
		}
		cmd := strings.ToLower(in)
		switch cmd {
		default:
			fmt.Println("Unknown cmd")
		case "quit", "exit", "q":
			break scanloop
		case "adddisk":
			fmt.Print("Enter path: ")
			if !scanner.Scan() {
				break scanloop
			}
			path := scanner.Text()
			absPath, err := filepath.Abs(path)
			if err != nil {
				fmt.Println("Error turning path into absolute path: " + err.Error())
				continue scanloop
			}
			disk, err := rubberhose.NewDisk(absPath)
			if err != nil {
				fmt.Println("Error opening disk: " + err.Error())
				continue scanloop
			}
			bs, err := disk.GetBlockSize()
			if err != nil {
				fmt.Println("Error reading disk block size: " + err.Error())
				continue scanloop
			}
			bc, err := disk.GetBlockCount()
			if err != nil {
				fmt.Println("Error reading disk block count: " + err.Error())
				continue scanloop
			}
			disks = append(disks, disk)
			fmt.Printf("Success! Disk num %d (Blocksize: %s, Blockcount: %d).\n", len(disks), ByteSizeToHumanReadable(bs), bc)
		case "createdisk":
			fmt.Print("Enter path: ")
			if !scanner.Scan() {
				break scanloop
			}
			path := scanner.Text()
			absPath, err := filepath.Abs(path)
			if err != nil {
				fmt.Println("Error turning path into absolute path: " + err.Error())
				continue scanloop
			}
			fmt.Print("Enter block size: ")
			if !scanner.Scan() {
				break scanloop
			}
			bs, err := bytesize.Parse([]byte(scanner.Text()))
			if err != nil {
				fmt.Println("Error parsing byte size: " + err.Error())
				continue scanloop
			}
			var blockCount int64
			var disk *rubberhose.Disk
			if _, err := os.Stat(absPath); err != nil {
				f, err := os.Create(absPath)
				if err != nil {
					fmt.Println("Error creating disk: " + err.Error())
					continue scanloop
				}
				fmt.Print("Enter block count: ")
				if !scanner.Scan() {
					break scanloop
				}
				bc, err := strconv.ParseUint(scanner.Text(), 10, 64)
				if err != nil {
					fmt.Println("Error parsing block count: " + err.Error())
					continue scanloop
				}
				blockCount = int64(bc)
				disk = rubberhose.NewDiskFromFile(f)
			} else {
				d, err := rubberhose.NewDisk(absPath)
				if err != nil {
					fmt.Println("Error opening disk: " + err.Error())
				}
				disk = d
			}
			err = disk.Write(int64(bs), blockCount)
			if err != nil {
				fmt.Println("Error writing disk: " + err.Error())
				continue scanloop
			}
			disks = append(disks, disk)
			fmt.Printf("Success! Disk num %d.\n", len(disks))
		case "addpartition":
			state, partition := getPartition(disks, scanner, false)
			switch state {
			case Break:
				break scanloop
			case Continue:
				continue scanloop
			}
			path, bd := partition.Expose()
			defer bd.Disconnect()
			fmt.Printf("Success! Partition exposed as %s! Blockcount: %d, Total Size: %s\n", path, partition.GetBlockCount(), ByteSizeToHumanReadable(partition.GetDataSize()))
		case "createpartition":
			fmt.Print("Enter disk num: ")
			if !scanner.Scan() {
				break scanloop
			}
			diskNum, err := strconv.Atoi(scanner.Text())
			if err != nil {
				fmt.Println("Error parsing disk num: " + err.Error())
				continue scanloop
			}
			if diskNum > len(disks) {
				fmt.Println("Invalid disk num")
				continue scanloop
			}
			diskNum--
			disk := disks[diskNum]
			password := ""
			pw := getPassword(&password, false)
			_, err = disk.GetPartition(pw)
			if err == nil {
				fmt.Println("A partition with this password already exists!")
				continue scanloop
			}
			fmt.Print("Enter block count: ")
			if !scanner.Scan() {
				break scanloop
			}
			blockCount, err := strconv.Atoi(scanner.Text())
			if err != nil {
				fmt.Println("Error parsing block count: " + err.Error())
				continue scanloop
			}
			partition, err := disk.WritePartition(pw, int64(blockCount))
			if err != nil {
				fmt.Println("Error writing partition: " + err.Error())
				continue scanloop
			}
			path, bd := partition.Expose()
			defer bd.Disconnect()
			fmt.Printf("Success! Partition exposed as %s!\n", path)
		case "delete":
			state, partition := getPartition(disks, scanner, true)
			switch state {
			case Break:
				break scanloop
			case Continue:
				continue scanloop
			}
			err := partition.Delete()
			if err != nil {
				fmt.Println("Error deleting partition: " + err.Error())
				continue scanloop
			}
			fmt.Println("Successfully deleted partition.")
		}
	}
}

type ReturnState int

const (
	Break ReturnState = iota
	Continue
	Nothing
)

func getPartition(disks []*rubberhose.Disk, scanner *bufio.Scanner, ignoreInvalidBlockStructure bool) (ReturnState, *rubberhose.Partition) {
	fmt.Print("Enter disk num: ")
	if !scanner.Scan() {
		return Break, nil
	}
	diskNum, err := strconv.Atoi(scanner.Text())
	if err != nil {
		fmt.Println("Error parsing disk num: " + err.Error())
		return Continue, nil
	}
	if diskNum > len(disks) {
		fmt.Println("Invalid disk num")
		return Continue, nil
	}
	diskNum--
	disk := disks[diskNum]
	password := ""
	pw := getPassword(&password, false)
	partition, err := disk.GetPartition(pw)
	if err != nil && !(err == rubberhose.InvalidBlockStructure && ignoreInvalidBlockStructure) {
		fmt.Println("Error opening partition: " + err.Error())
		return Continue, nil
	}
	return Nothing, partition
}

func ByteSizeToHumanReadable(size int64) string {
	const unit = 1000
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(size)/float64(div), "kMGTPE"[exp])
}
