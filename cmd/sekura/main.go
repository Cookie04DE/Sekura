package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	rubberhose "github.com/Cookie04DE/RubberHose"
	"github.com/dop251/buse"
	"github.com/inhies/go-bytesize"
	"golang.org/x/sys/unix"
)

var counter int

func main() {
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
			if _, err := os.Stat(absPath); err != nil {
				fmt.Println("Unknown file")
				continue scanloop
			}
			f, err := os.OpenFile(absPath, os.O_RDWR, 0755)
			if err != nil {
				fmt.Println("Error opening file: " + err.Error())
				continue scanloop
			}
			disks = append(disks, rubberhose.NewDisk(f))
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
			var file *os.File
			var blockCount int64
			if _, err := os.Stat(absPath); err != nil {
				f, err := os.Create(absPath)
				if err != nil {
					fmt.Println("Error creating disk: " + err.Error())
					continue scanloop
				}
				file = f
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
			} else {
				f, err := os.Open(absPath)
				if err != nil {
					fmt.Println("Error opening disk: " + err.Error())
					continue scanloop
				}
				file = f
			}
			disk := rubberhose.NewDisk(file)
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
			for {
				path, bd, err := mount(*partition)
				if err != nil {
					continue
				}
				defer bd.Disconnect()
				fmt.Printf("Success! Partition mounted as %s!\n", path)
				break
			}
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
			for {
				path, bd, err := mount(*partition)
				if err != nil {
					continue
				}
				defer bd.Disconnect()
				fmt.Printf("Success! Partition mounted as %s!\n", path)
				break
			}
		}
	}
}

func mount(p rubberhose.Partition) (string, *buse.Device, error) {
	path := fmt.Sprintf("/dev/nbd%d", counter)
	counter++
	bd, err := buse.NewDevice(path, p.GetDataSize(), p)
	go func() {
		err := bd.Run()
		if err != nil {
			log.Fatal("Hallo ", err)
		}
	}()
	return path, bd, err
}
