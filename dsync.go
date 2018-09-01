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
	"io/ioutil"
	"bytes"
	"strings"
)

var (
	mode    string
	verbose bool

	// generate mode
	path         string
	generateFile string
	skipDirFile  string

	// compare mode
	sourceFile string
	destFile   string
	resultFile string
)

func init() {
	flag.StringVar(&mode, "mode", "", "-mode=generate")
	flag.BoolVar(&verbose, "v", false, "-v")

	// generate mode
	flag.StringVar(&path, "path", "", "-path=/home/USER1/DIR")
	flag.StringVar(&generateFile, "gen", "", "-gen=/home/USER2/gen_file")
	flag.StringVar(&skipDirFile, "skip", "", "-skip=/home/USER2/skip_file , only support skip directory")

	// compare mode
	flag.StringVar(&sourceFile, "src", "", "-src=/home/USER2/source_file")
	flag.StringVar(&destFile, "dst", "", "-dst=/home/USER2/dst_file")
	flag.StringVar(&resultFile, "result", "", "-result=/home/USER2/result_file")
}

func main() {
	flag.Parse()

	switch mode {
	case "generate":
		generate()
	case "compare":
		compare()
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

	var skipDirName map[string]struct{}
	if len(skipDirFile) != 0 {
		var err error
		skipDirName, err = readSkipFile(skipDirFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read skip file error:", err)
			os.Exit(1)
		}
	} else {
		skipDirName = make(map[string]struct{})
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
				if _, ok := skipDirName[info.Name()]; ok {
					return filepath.SkipDir
				}
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

func readSkipFile(file string) (map[string]struct{}, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(data)
	result := make(map[string]struct{})
	for {
		line, err := buf.ReadString('\n')
		//FIXME should trim if filename starts/ends with space??
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		result[line] = struct{}{}
		if err != nil {
			if err == io.EOF {
				return result, nil
			}
			return nil, err
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
	defer f.Close()
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

func readFromFile(file string) (*GeneratedStore, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	store := new(GeneratedStore)
	err = json.Unmarshal(data, store)
	if err != nil {
		return nil, err
	}
	return store, nil
}

func compare() {
	if len(sourceFile) == 0 {
		fmt.Fprintln(os.Stderr, "source file is empty")
		os.Exit(1)
	}
	if len(destFile) == 0 {
		fmt.Fprintln(os.Stderr, "dest file is empty")
		os.Exit(1)
	}
	if len(resultFile) == 0 {
		fmt.Fprintln(os.Stderr, "result file is empty")
		os.Exit(1)
	}

	sourceStore, err := readFromFile(sourceFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open source file error:", err)
		os.Exit(1)
	}
	destStore, err := readFromFile(destFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open dest file error:", err)
		os.Exit(1)
	}

	if sourceStore.IsDir != destStore.IsDir {
		fmt.Fprintln(os.Stderr, "source type of file/dir is different from dst one's")
		os.Exit(1)
	}

	sourceMap := make(map[string]FileValue)
	dstMap := make(map[string]FileValue)
	for _, v := range sourceStore.FileList {
		_, ok := sourceMap[v.RelPath]
		if ok {
			fmt.Fprintln(os.Stderr, "source file duplicated record:", v.RelPath)
			os.Exit(1)
		}
		sourceMap[v.RelPath] = v
	}
	for _, v := range destStore.FileList {
		_, ok := dstMap[v.RelPath]
		if ok {
			fmt.Fprintln(os.Stderr, "dest file duplicated record:", v.RelPath)
			os.Exit(1)
		}
		dstMap[v.RelPath] = v
	}

	result := new(CompareResult)

	// compare
	var dupList []string
	for k, srcV := range sourceMap {
		dstV, ok := dstMap[k]
		if ok {
			dupList = append(dupList, k)

			// check same
			if srcV.Size != dstV.Size || srcV.Sha256 != dstV.Sha256 {
				result.Diff = append(result.Diff, CompareDifferentValue{
					SrcValue: srcV,
					DstValue: dstV,
				})
			}
		}
	}
	// remove dup
	for _, v := range dupList {
		delete(sourceMap, v)
		delete(dstMap, v)
	}

	// find only
	for _, v := range sourceMap {
		result.SrcOnly = append(result.SrcOnly, v)
	}
	for _, v := range dstMap {
		result.DstOnly = append(result.DstOnly, v)
	}

	// compare result
	if verbose {
		fmt.Println(result)
	}
	err = writeCompareResult(resultFile, result)
	if err != nil {
		fmt.Fprintln(os.Stderr, "write compare result error:", err)
		os.Exit(1)
	}
}

func writeCompareResult(file string, result *CompareResult) error {
	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err != nil {
		return err
	}
	return nil
}

type CompareResult struct {
	Diff    []CompareDifferentValue `json:"diff"`
	SrcOnly []FileValue             `json:"src_only"`
	DstOnly []FileValue             `json:"dst_only"`
}

type CompareDifferentValue struct {
	SrcValue FileValue `json:"src"`
	DstValue FileValue `json:"dst"`
}
