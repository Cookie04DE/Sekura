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
	"github.com/inhies/go-bytesize"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/sys/unix"
)

func addCommonFlags(fs *flag.FlagSet) (*bool, *string, *string) {
	parsable := fs.Bool("parsable", false, "Provide output in machine parsable output instead of human readable format")
	disk := fs.String("disk", "", "The sekura disk to work on")
	password := fs.String("password", "", "The password of the partition to work on (can also be provided interactively)")
	return parsable, disk, password
}

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
	passwordBytes, err := terminal.ReadPassword(int(syscall.Stdin))
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
	flag.Parse()
	if *standalone {
		runStandaloneMode()
		return
	}
	addCmd := flag.NewFlagSet("add", flag.ExitOnError)
	parsable, disk, password := addCommonFlags(addCmd)
	if len(os.Args) == 1 {
		flag.Usage()
		return
	}
	conn, err := net.Dial("unix", "/run/sekura.sock")
	if err != nil {
		log.Fatal("Error opening connection to daemon: " + err.Error())
	}
	rubberhose.RegisterGob()
	d := gob.NewDecoder(conn)
	e := gob.NewEncoder(conn)
	switch os.Args[1] {
	default:
		flag.Usage()
		return
	case "add":
		addCmd.Parse(os.Args[2:])
		if *disk == "" {
			log.Fatal("Please provide a disk with the -disk flag")
		}
		pw := getPassword(password, *parsable)
		err = e.Encode(&rubberhose.Request{ID: rubberhose.AddRequestID, Data: rubberhose.AddRequest{DiskPath: *disk, Password: pw}})
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
	}
}

func runStandaloneMode() {
	if unix.Geteuid() != 0 {
		log.Fatal("Sekura requires root permissions to work")
	}
	disks := []rubberhose.Disk{}
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
		case "quit", "q":
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
			disks = append(disks, *disk)
			fmt.Printf("Success! Disk num %d.\n", len(disks))
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
			bs, err := bytesize.Parse(scanner.Text())
			if err != nil {
				fmt.Println("Error parsing byte size: " + err.Error())
				continue scanloop
			}
			var blockCount int64
			var disk rubberhose.Disk
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
				disk = *d
			}
			err = disk.Write(int64(bs), blockCount)
			if err != nil {
				fmt.Println("Error writing disk: " + err.Error())
				continue scanloop
			}
			disks = append(disks, disk)
			fmt.Printf("Success! Disk num %d.\n", len(disks))
		case "addpartition":
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
			fmt.Print("Enter password: ")
			if !scanner.Scan() {
				break scanloop
			}
			partition, err := disk.GetPartition(scanner.Text())
			if err != nil {
				fmt.Println("Error opening partition: " + err.Error())
				continue scanloop
			}
			path, bd := partition.Mount()
			defer bd.Disconnect()
			fmt.Printf("Success! Partition mounted as %s!\n", path)
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
			fmt.Print("Enter password: ")
			if !scanner.Scan() {
				break scanloop
			}
			password := scanner.Text()
			_, err = disk.GetPartition(password)
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
			partition, err := disk.WritePartition(password, int64(blockCount))
			if err != nil {
				fmt.Println("Error writing partition: " + err.Error())
				continue scanloop
			}
			path, bd := partition.Mount()
			defer bd.Disconnect()
			fmt.Printf("Success! Partition mounted as %s!\n", path)
		}
	}
}
