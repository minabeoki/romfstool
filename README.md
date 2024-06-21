# romfs tool

## What?

Show the informations of [RomFS](https://docs.kernel.org/filesystems/romfs.html) image file.
And extract files from RomFS image file.

## Usage

Show the romfs informations:

    > romfstool romfs_image.bin
    volume name: VolumeName
    romfs size: 9792 (0x2640)
    
        offset      size  filename
    0x00000020         0  /./
    0x00000040         0  /.. -> ./
    0x00000060      1234  /fuga.bin
    0x00000560      8192  /hoge.bin
    0x00002580         0  /dir/
    0x000025a0         0  /dir/. -> dir/
    0x000025c0         0  /dir/.. -> ./
    0x000025e0         8  /dir/bbb.txt
    0x00002610         4  /dir/aaa.txt

Extract files:

    > romfstool -x destdir romfs_image.bin
    volume name: VolumeName
    romfs size: 9792 (0x2640)
    extract: destdir
    
        offset      size  filename
    0x00000020         0  /./
    0x00000040         0  /.. -> ./
    0x00000060      1234  /fuga.bin
    0x00000560      8192  /hoge.bin
    0x00002580         0  /dir/
    0x000025a0         0  /dir/. -> dir/
    0x000025c0         0  /dir/.. -> ./
    0x000025e0         8  /dir/bbb.txt
    0x00002610         4  /dir/aaa.txt
    
    > ls -FR destdir
    dir/            fuga.bin        hoge.bin
    
    destdir/dir:
    aaa.txt         bbb.txt
