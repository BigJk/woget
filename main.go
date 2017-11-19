package main

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var wowPath string

var success chan string
var failed chan string

func main() {
	// Get exe folder
	filename, _ := os.Executable()
	filename = strings.Replace(filename, ".exe", "", -1)
	filename += ".path"

	// Parse Args
	args := os.Args
	if len(args) <= 1 {
		fmt.Println("Arguments missing")
		return
	}

	if args[1] == "set" {
		ioutil.WriteFile(filename, []byte(strings.Join(args[2:len(args)], " ")), 0777)
		fmt.Println("Path set")
		return
	}

	// Read wow path
	path, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Println("World Of Warcraft Path missing. Please use 'woget set \"C:\\your\\path\"'")
		return
	}
	wowPath = string(path)

	// Create folder if not existing
	if _, err := ioutil.ReadDir(wowPath + "\\Interface\\AddOns"); err != nil {
		os.MkdirAll(wowPath+"\\Interface\\AddOns", os.ModePerm)
	}

	success = make(chan string, 100)
	failed = make(chan string, 100)

	for i := 1; i < len(args); i++ {
		go downloadAddonAsync(args[i])
	}

	for len(success)+len(failed) < len(args)-1 {
		time.Sleep(time.Second)
	}

	if len(success) > 0 {
		fmt.Println("\nSuccessful Installed:")
		for len(success) > 0 {
			fmt.Printf(" > %s\n", <-success)
		}
		if len(failed) > 0 {
			fmt.Print("\n")
		}
	}

	if len(failed) > 0 {
		fmt.Println("Errors:")
		for len(failed) > 0 {
			fmt.Printf(" > %s\n", <-failed)
		}
	}
}

func downloadAddonAsync(name string) {
	if downloadAddon(name) {
		success <- name
	} else {
		failed <- name
	}
}

func downloadAddon(name string) bool {
	doc, err := goquery.NewDocument(fmt.Sprintf("https://www.curseforge.com/wow/addons/%s/download", name))
	if err != nil {
		fmt.Printf("<%s> Downloading Failed... (Unknown Reason)\n", name)
		return false
	}

	if strings.Contains(doc.Text(), "We were unable to find the page or file you were looking for") {
		fmt.Printf("<%s> Downloading Failed... (Addon Not Found)\n", name)
		return false
	}

	link := doc.Find("a.download__link").AttrOr("href", "")
	if len(link) == 0 {
		fmt.Printf("<%s> Downloading Failed... (Download Link Not Found)\n", name)
		return false
	}

	fileName := fmt.Sprintf("%d.zip", rand.Int())
	out, err := os.Create(fileName)
	defer out.Close()
	defer os.Remove(fileName)

	resp, err := http.Get(fmt.Sprintf("https://www.curseforge.com%s", link))
	if err != nil {
		fmt.Printf("<%s> Downloading Failed... (Download Currently Not Available)\n", name)
		return false
	}

	fmt.Printf("<%s> Downloading...\n", name)
	defer resp.Body.Close()
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		fmt.Printf("<%s> Downloading Failed... (Download Aborted)\n", name)
		return false
	}

	fmt.Printf("<%s> Unzipping...\n", name)
	_, err = unzip(fileName, wowPath+"\\Interface\\AddOns")
	if err != nil {
		fmt.Printf("<%s> Downloading Failed... (Unzipping Failed)\n", name)
		return false
	}

	fmt.Printf("<%s> DONE\n", name)
	return true
}

func unzip(src, dest string) ([]string, error) {
	var filenames []string

	r, err := zip.OpenReader(src)
	if err != nil {
		return filenames, err
	}
	defer r.Close()

	for _, f := range r.File {

		rc, err := f.Open()
		if err != nil {
			return filenames, err
		}
		defer rc.Close()

		fpath := filepath.Join(dest, f.Name)
		filenames = append(filenames, fpath)

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
		} else {
			var fdir string
			if lastIndex := strings.LastIndex(fpath, string(os.PathSeparator)); lastIndex > -1 {
				fdir = fpath[:lastIndex]
			}
			err = os.MkdirAll(fdir, os.ModePerm)
			if err != nil {
				log.Fatal(err)
				return filenames, err
			}
			f, err := os.OpenFile(
				fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return filenames, err
			}
			defer f.Close()

			_, err = io.Copy(f, rc)
			if err != nil {
				return filenames, err
			}

		}
	}
	return filenames, nil
}
