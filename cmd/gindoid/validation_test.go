package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func writeTmpFile(filename string, content string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.WriteString(file, content)
	if err != nil {
		return err
	}
	return file.Sync()
}

func TestLicenseFromFile(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "test_gindoi_licfromfile")
	if err != nil {
		t.Fatalf("Error creating tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	licfile := filepath.Join(tmpDir, "lic.json")
	licURL := "lic url"
	content := fmt.Sprintf(`[{"URL": "%s", "Name":  "lic name", "Alias": ["alias name"]}]`, licURL)
	err = writeTmpFile(licfile, content)
	if err != nil {
		t.Fatalf("Error creating json file: '%s'", err.Error())
	}

	liclist, err := licenseFromFile(licfile)
	if err != nil {
		t.Fatalf("Could not load custom license file: '%s'", err.Error())
	}
	if len(liclist) != 1 {
		t.Fatalf("Unexpected license list length: '%d'", len(liclist))
	}
	if licURL != liclist[0].URL {
		t.Fatalf("Unexpected license content: '%s'/'%s'", licURL, liclist[0].URL)
	}
}

func TestCleanupcompstr(t *testing.T) {
	instr := "  aLLcasEs  "
	expected := "allcases"
	outstr := cleancompstr(instr)
	if outstr != expected {
		t.Fatalf("Error string cleanup: '%s' expected: '%s'", outstr, expected)
	}
}
