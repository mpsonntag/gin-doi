package main

import (
	"io/ioutil"
	"testing"

	"os"
	"path/filepath"
	"strings"

	"github.com/G-Node/gin-core/gin"
)

func TestPrepDir(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "TestGin")
	defer os.RemoveAll(tmpDir)
	if err != nil {
		t.Log("[Err] Could nor create temporary directory for prep test")
		t.Fail()
		return
	}
	dp := MockDOIProvider{}
	ds := LocalStorage{Path: tmpDir, DProvider: dp}

	if err := ds.prepDir("test1", nil); err != nil {
		t.Logf("[error] Could not prepare directory: %+v", err)
		t.Fail()
		return
	}
	fp, err := os.Open(filepath.Join(tmpDir, "test1", ".htaccess"))
	if err != nil {
		t.Logf("[Err] Could not open .htaccess: %s", err.Error())
		return
	}
	ct, err := ioutil.ReadAll(fp)
	if err != nil {
		t.Logf("[Err] Could not read form .htaccess: %s", err.Error())
		return
	}
	if string(ct) == "deny from all" {
		t.Log("[OK] Prepare Dir works")
		return
	}
	t.Fail()
	return
}

func fileThere(fn string, tmpDir string, t *testing.T) {
	_, err := os.Open(filepath.Join(tmpDir, "123", fn))
	if err == nil {
		t.Log("[OK] Put creates " + fn)
	} else {
		t.Logf("[Err] Put creates no %s: %+v", fn, err)
		t.Fail()
	}
}
func TestPut(t *testing.T) {
	//log.SetLevel(log.DebugLevel)
	tmpDir, err := ioutil.TempDir("", "TestGin")
	defer os.RemoveAll(tmpDir)
	if err != nil {
		t.Log("[Err] Could nor create temporary directory for prep test")
		t.Fail()
		return
	}
	ds := &MockDataSource{validDOIFile: true, Berry: DOIRegInfo{}}
	ls := LocalStorage{Path: tmpDir, Source: ds, DProvider: MockDOIProvider{},
		MServer: &MailServer{}}
	dReq := DOIReq{}
	dReq.User.MainOId.Email = &gin.Email{Email: "123"}

	mJob := DOIJob{Name: "123", Source: "nowhere", Request: dReq}

	ls.Put(mJob)

	fileThere("123.zip", tmpDir, t)
	fileThere("doi.xml", tmpDir, t)
	fileThere(".htaccess", tmpDir, t)
	if strings.Contains(ds.calls[0], "nowhere") {
		t.Log("[OK] Get was called properly")
	} else {
		t.Log("[ERR] Get was not called properly")
		t.Fail()
	}
}