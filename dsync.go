package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"crypto/sha256"
	"io"
	"errors"
	"encoding/json"
)

var (
	mode    string
	verbose bool

	// for generate mode
	path         string
	generateFile string
)

func init() {
	flag.StringVar(&mode, "mode", "", "-mode=generate")
	flag.BoolVar(&verbose, "v", false, "-v")

	flag.StringVar(&path, "path", "", "-path=/home/USER1/DIR")
	flag.StringVar(&generateFile, "gen", "", "-gen=/home/USER2/gen_file")
}

func main() {
	flag.Parse()

	switch mode {
	case "generate":
		generate()
	case "compare":
		panic("not implemented")
	default:
		fmt.Fprintln(os.Stderr, "unknown mode:", mode)
		fmt.Fprintln(flag.CommandLine.Output(), "usage:")
		flag.PrintDefaults()
		os.Exit(1)
	}
}

type GeneratedStore struct {
	IsDir    bool        `json:"dir"`
	FileList []FileValue `json:"files"`
}

type FileValue struct {
	RelPath string `json:"path"`
	Size    int64  `json:"size"`
	Sha256  string `json:"sha256"`
}

func (this FileValue) String() string {
	return fmt.Sprint("sha256:", this.Sha256, " ", this.Size, " ", this.RelPath)
}

func (this *GeneratedStore) Print() {
	fmt.Println("=====>>>>")
	var t = "file"
	if this.IsDir {
		t = "dir"
	}
	fmt.Println("Store type:", t)
	for _, v := range this.FileList {
		fmt.Println(v)
	}
	fmt.Println("<<<<=====")
}

func generate() {
	if len(path) == 0 {
		fmt.Fprintln(os.Stderr, "path is empty")
		os.Exit(1)
	}
	if len(generateFile) == 0 {
		fmt.Fprintln(os.Stderr, "generate file is empty")
		os.Exit(1)
	}

	info, err := os.Lstat(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open file error:", err)
		os.Exit(1)
	}

	store := new(GeneratedStore)

	if !info.IsDir() {
		// walk file
		store.IsDir = false
		result, err := genFileSha256(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "generate sha256 digest error:", err)
			os.Exit(1)
		}
		store.FileList = append(store.FileList, FileValue{
			RelPath: info.Name(),
			Size:    info.Size(),
			Sha256:  result,
		})
		// need write to file
		err = writeToFile(store, generateFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "write generated file error:", err)
			os.Exit(1)
		}
	} else {
		// walk directory
		store.IsDir = true
		rootPath, err := filepath.Abs(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "get absolute path error:", err)
			os.Exit(1)
		}
		err = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return errors.New("p1:" + err.Error())
			}
			if info.IsDir() {
				return nil
			}
			realPath, err := filepath.Abs(path)
			if err != nil {
				return errors.New("p2:" + err.Error())
			}
			relPath, err := filepath.Rel(rootPath, realPath)
			if err != nil {
				return errors.New("p3:" + err.Error())
			}
			if verbose {
				fmt.Println("reading file:", realPath)
			}
			result, err := genFileSha256(path)
			if err != nil {
				return errors.New("p4:" + err.Error())
			}
			store.FileList = append(store.FileList, FileValue{
				RelPath: relPath,
				Size:    info.Size(),
				Sha256:  result,
			})
			return nil
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "walk file/dir error:", err)
			os.Exit(1)
		}
		// need write to file
		err = writeToFile(store, generateFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "write generated file error:", err)
			os.Exit(1)
		}
	}
}

func genFileSha256(file string) (string, error) {
	digest := sha256.New()
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, err = io.Copy(digest, f)
	if err != nil {
		return "", err
	}
	result := digest.Sum(nil)
	return fmt.Sprintf("%x", result), nil
}

func writeToFile(store *GeneratedStore, dst string) error {
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	data, err := json.Marshal(store)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err != nil {
		return err
	}
	return nil
}
