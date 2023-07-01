package main

import (
	"encoding/binary"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"math/bits"
	"slices"
)

var (
	webmMagic     = []byte{0x1A, 0x45, 0xDF, 0xA3}
	clusterID     = []byte{0x1F, 0x43, 0xB6, 0x75}
	simpleBlockID = []byte{0xA3}
	blockGroupID  = []byte{0xA0}
)

func readVint(r io.Reader) (uint64, int, error) {
	firstByteBytes := make([]byte, 1)
	_, err := io.ReadAtLeast(r, firstByteBytes, 1)
	if err != nil {
		return 0, 0, err
	}
	firstByte := firstByteBytes[0]
	lengthOfSize := bits.LeadingZeros8(firstByte)
	firstByte ^= 1 << (7 - lengthOfSize)
	if lengthOfSize == 0 {
		return uint64(firstByte), 1, nil
	}
	additionalBytes := make([]byte, lengthOfSize)
	_, err = io.ReadAtLeast(r, additionalBytes, lengthOfSize)
	if err != nil {
		return 0, 0, err
	}
	b := append(make([]byte, 7-lengthOfSize), firstByte)
	b = append(b, additionalBytes...)

	return binary.BigEndian.Uint64(b), lengthOfSize + 1, nil
}

//func skipWebmHeader(r io.Reader) error {
//	magic := make([]byte, len(webmMagic))
//	_, err := r.Read(magic)
//	if err != nil {
//		return errors.Wrap(err, "read webm magic")
//	}
//	if !slices.Equal(magic, webmMagic) {
//		return fmt.Errorf("magic %v does not match webm", magic)
//	}
//	ebmlHeaderSize, _, err := readVint(r)
//	if err != nil {
//		return errors.Wrap(err, "read ebml header size")
//	}
//	_, err = r.Read(make([]byte, ebmlHeaderSize+4))
//	if err != nil {
//		return errors.Wrap(err, "read ebml header")
//	}
//	_, _, err = readVint(r)
//	if err != nil {
//		return errors.Wrap(err, "read matroska segment size")
//	}
//	_, err = r.Read(make([]byte, 4))
//	if err != nil {
//		return errors.Wrap(err, "read seekhead id")
//	}
//	seekHeadSize, _, err := readVint(r)
//	if err != nil {
//		return errors.Wrap(err, "read seekhead size")
//	}
//	_, err = r.Read(make([]byte, seekHeadSize+4))
//	if err != nil {
//		return errors.Wrap(err, "read seekhead")
//	}
//	segmentInformationSize, _, err := readVint(r)
//	if err != nil {
//		return errors.Wrap(err, "read matroska segment information size")
//	}
//	_, err = r.Read(make([]byte, segmentInformationSize+4))
//	if err != nil {
//		return errors.Wrap(err, "read matroska segment information")
//	}
//	tracksSize, _, err := readVint(r)
//	if err != nil {
//		return errors.Wrap(err, "read matroska tracks size")
//	}
//	_, err = r.Read(make([]byte, tracksSize+4))
//	if err != nil {
//		return errors.Wrap(err, "read matroska tracks information")
//	}
//	_, _, err = readVint(r)
//	if err != nil {
//		return errors.Wrap(err, "read cluster size")
//	}
//	_, err = r.Read(make([]byte, 1))
//	if err != nil {
//		return errors.Wrap(err, "read timecode byte")
//	}
//	timecodeSize, _, err := readVint(r)
//	if err != nil {
//		return errors.Wrap(err, "read timecode size")
//	}
//	_, err = r.Read(make([]byte, timecodeSize))
//	if err != nil {
//		return errors.Wrap(err, "read timecode")
//	}
//
//	return nil
//}

func skipWebmHeader(r io.Reader) error {
	magic := make([]byte, len(webmMagic))
	_, err := io.ReadAtLeast(r, magic, len(webmMagic))
	if err != nil {
		return errors.Wrap(err, "read webm magic")
	}
	if !slices.Equal(magic, webmMagic) {
		return fmt.Errorf("magic [% x] does not match webm", magic)
	}
	currentI := 0
	singleByte := make([]byte, 1)
	for {
		_, err = io.ReadAtLeast(r, singleByte, 1)
		if err != nil {
			return errors.Wrap(err, "read single byte")
		}
		if singleByte[0] == clusterID[currentI] {
			currentI++
			if currentI == len(clusterID) {
				break
			}
		} else {
			currentI = 0
		}
	}
	return nil
}

func readWebm(r io.Reader, data chan []byte, done chan error) error {
	err := skipWebmHeader(r)
	if err != nil {
		return errors.Wrap(err, "read webm header")
	}

	go func() {
		defer close(data)
		defer close(done)
		skipClusterHeader := true

		for {
			innerDone := make(chan error, 1)

			if !skipClusterHeader {
				blockID := make([]byte, 4)
				_, err = io.ReadAtLeast(r, blockID, 4)
				if !slices.Equal(blockID, clusterID) {
					done <- fmt.Errorf("id [% x] does not match cluster id", blockID)
					return
				}
			} else {
				skipClusterHeader = false
			}

			err = readCluster(r, data, innerDone)
			if err != nil {
				done <- errors.Wrap(err, "read cluster")
				return
			}
			if err = <-innerDone; err != nil {
				if !errors.Is(err, io.EOF) {
					done <- errors.Wrap(err, "read cluster")
				}
				return
			}
		}
	}()

	return nil
}

func readCluster(r io.Reader, data chan []byte, done chan error) error {
	clusterSize, _, err := readVint(r)
	if err != nil {
		return errors.Wrap(err, "read cluster size")
	}
	_, err = io.ReadAtLeast(r, make([]byte, 1), 1)
	if err != nil {
		return errors.Wrap(err, "read timecode byte")
	}
	timecodeSize, timecodeSizeSize, err := readVint(r)
	if err != nil {
		return errors.Wrap(err, "read timecode size")
	}
	_, err = io.ReadAtLeast(r, make([]byte, timecodeSize), int(timecodeSize))
	if err != nil {
		return errors.Wrap(err, "read timecode")
	}
	currentlyRead := uint64(1+timecodeSizeSize) + timecodeSize
	go func() {
		defer close(done)
		for {
			blockData, n, err := readBlockData(r)
			if err != nil {
				if errors.Is(err, io.EOF) {
					done <- io.EOF
				}
				done <- errors.Wrap(err, "read block data")
				return
			}
			data <- blockData
			currentlyRead += uint64(n)
			if currentlyRead >= clusterSize {
				return
			}
		}
	}()
	return nil
}

func readBlockData(r io.Reader) ([]byte, int, error) {
	blockID := make([]byte, len(simpleBlockID))
	_, err := io.ReadAtLeast(r, blockID, len(simpleBlockID))
	if err != nil {
		return nil, 0, errors.Wrap(err, "read block id")
	}
	if !slices.Equal(blockID, simpleBlockID) {
		if slices.Equal(blockID, blockGroupID) {
			return nil, 0, io.EOF
		}
		return nil, 0, fmt.Errorf("id [% x] does not match simple block id", blockID)
	}
	blockSizeU, blockSizeSize, err := readVint(r)
	if err != nil {
		return nil, 0, errors.Wrap(err, "read block size")
	}
	blockSize := int(blockSizeU)
	_, n, err := readVint(r)
	if err != nil {
		return nil, 0, errors.Wrap(err, "read track num")
	}
	block := make([]byte, blockSize-n)
	_, err = io.ReadAtLeast(r, block, blockSize-n)
	if err != nil {
		return nil, 0, errors.Wrap(err, "read block")
	}
	return block[3:], len(simpleBlockID) + blockSizeSize + blockSize, nil
}
