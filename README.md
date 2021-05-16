# What is Sekura?
Sekura is an Encryption tool that's heavily inspired by the [Rubberhose file system](https://en.wikipedia.org/wiki/Rubberhose_(file_system)).

It allows for multiple, independent file systems on a single disk whose existence can only be verified if you posses the correct password.

# Requirements

1. A working go installation
2. The [nbd](https://en.wikipedia.org/wiki/Network_block_device) kernel module

# How to use

1. Clone this repository
2. `cd` into `cmd/Sekura` and run `go install .`

Note: the following steps require root permissions

3. Run `modprobe nbd` to start the [nbd](https://en.wikipedia.org/wiki/Network_block_device) kernel module
4. Run `sekura` to enter the command line

# Command line
## Commands:
### quit:
This quits Sekura. **Don't quit if you still want to read or write to the partitions**
### createDisk:
This creates and adds a disk for use with Sekura.

**Warning:** this command will **overwrite** the file at the provided path if it already exists.

It asks you for a block size and a block count. You may enter the size as a number with a suffix (e.g "4mb", "10GB", "1tb"). The final size of the disk will be the size multiplied by the count plus 20 bytes for the disk header.

The more blocks you choose the more file systems can fit on that disk. The block size needs to be a minimum of 32 bytes to accommodate the block header, but more size is needed to actually store data.
### addDisk:
This adds a disk previously created by `createDisk` to read and write partitions on it.
### createPartition:
This creates a partition on a previously added/created disk and adds it.

**Warning:** mount all partitions on the disk before using this command as you might otherwise encounter **data loss**.

Sekura will ask you for the number of the disk you want to create the partition on.

You are then asked to enter a password and Sekura makes sure there isn't already a partition with that password on the disk.

After that Sekura will ask you for the amount of blocks you want to allocate for this partition. The resulting size of the partition is `(blockSize - 32) * blockAmount`.
### addPartition:
This adds a previously created partition.

Sekura will ask you for the number of the disk and a password.

# How to use added partitions:

Once a partition is created/added you will receive the path to the block device (e.g. "/dev/nbd0").

Initially there is no filesystem on this device, so you need to create one. (You only need to do this once)

Note: The following steps require root permissions.

Example: `mkfs.ext4 /dev/nbd0`

Once you created your file system you need to mount it.

Example: `mount /dev/nbd0 /mnt`

Now your partition is mounted and you can use it like any other file system.