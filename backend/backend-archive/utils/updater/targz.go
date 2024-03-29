// Copyright 2023 Krisna Pranav, Sankar-2006. All rights reserved.
// Use of this source code is governed by a Apache-2.0 License
// license that can be found in the LICENSE file

// tar functioanlities
package updater

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func TarGZExtract(inputFilePath, outputFilePath string) (err error) {
	outputFilePath = stripTrailingSlashes(outputFilePath)
	inputFilePath, outputFilePath, err = makeAbsolute(inputFilePath, outputFilePath)
	if err != nil {
		return err
	}
	undoDir, err := mkdirAll(outputFilePath, 0750)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			undoDir()
		}
	}()

	return extract(inputFilePath, outputFilePath)
}

func mkdirAll(dirPath string, perm os.FileMode) (func(), error) {
	var undoDir string

	for p := dirPath; ; p = filepath.Dir(p) {
		finfo, err := os.Stat(p)
		if err == nil {
			if finfo.IsDir() {
				break
			}

			finfo, err = os.Lstat(p)
			if err != nil {
				return nil, err
			}

			if finfo.IsDir() {
				break
			}

			return nil, fmt.Errorf("mkdirAll (%s): %v", p, syscall.ENOTDIR)
		}

		if os.IsNotExist(err) {
			undoDir = p
		} else {
			return nil, err
		}
	}

	if undoDir == "" {
		return func() {}, nil
	}

	if err := os.MkdirAll(dirPath, perm); err != nil {
		return nil, err
	}

	return func() {
		if err := os.RemoveAll(undoDir); err != nil {
			panic(err)
		}
	}, nil
}

func stripTrailingSlashes(path string) string {
	if len(path) > 0 && path[len(path)-1] == '/' {
		path = path[0 : len(path)-1]
	}

	return path
}

func makeAbsolute(inputFilePath, outputFilePath string) (string, string, error) {
	inputFilePath, err := filepath.Abs(inputFilePath)
	if err == nil {
		outputFilePath, err = filepath.Abs(outputFilePath)
	}

	return inputFilePath, outputFilePath, err
}

func writeTarGz(path string, tarWriter *tar.Writer, fileInfo os.FileInfo, subPath string) error {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return err
	}

	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Error closing file: %s\n", err)
		}
	}()

	evaledPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}

	subPath, err = filepath.EvalSymlinks(subPath)
	if err != nil {
		return err
	}

	link := ""
	if evaledPath != path {
		link = evaledPath
	}

	header, err := tar.FileInfoHeader(fileInfo, link)
	if err != nil {
		return err
	}
	header.Name = evaledPath[len(subPath):]

	err = tarWriter.WriteHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(tarWriter, file)
	if err != nil {
		return err
	}

	return err
}

func extract(filePath string, directory string) error {
	file, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return err
	}

	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Error closing file: %s\n", err)
		}
	}()

	gzipReader, err := gzip.NewReader(bufio.NewReader(file))
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	type DirInfo struct {
		Path   string
		Header *tar.Header
	}

	postExtraction := []DirInfo{}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		fileInfo := header.FileInfo()
		if strings.Contains(fileInfo.Name(), "..") {
			continue
		}
		dir := filepath.Join(directory, filepath.Dir(header.Name))
		filename := filepath.Join(dir, fileInfo.Name())

		if fileInfo.IsDir() {
			if err := os.MkdirAll(filename, 0750); err != nil {
				return err
			}

			os.Chown(filename, header.Uid, header.Gid)

			postExtraction = append(postExtraction, DirInfo{filename, header})
			continue
		}

		if !fileInfo.IsDir() && !isDir(dir) {
			err = os.MkdirAll(dir, 0750)
			if err != nil {
				return err
			}
		}

		file, err := os.Create(filepath.Clean(filename))
		if err != nil {
			return err
		}

		writer := bufio.NewWriter(file)

		buffer := make([]byte, 4096)
		for {
			n, err := tarReader.Read(buffer)
			if err != nil && err != io.EOF {
				panic(err)
			}
			if n == 0 {
				break
			}

			_, err = writer.Write(buffer[:n])
			if err != nil {
				return err
			}
		}

		err = writer.Flush()
		if err != nil {
			return err
		}

		err = file.Close()
		if err != nil {
			return err
		}

		os.Chmod(filename, os.FileMode(header.Mode))
		os.Chtimes(filename, header.AccessTime, header.ModTime)
		os.Chown(filename, header.Uid, header.Gid)
	}

	if len(postExtraction) > 0 {
		for _, dir := range postExtraction {
			os.Chtimes(dir.Path, dir.Header.AccessTime, dir.Header.ModTime)
			os.Chmod(dir.Path, dir.Header.FileInfo().Mode().Perm())
		}
	}

	return nil
}
