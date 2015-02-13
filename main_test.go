package main

import (
	"fmt"
	"testing"
)

func TestParseVerbs(t *testing.T) {
	reg := "%.02f %s blah %d"
	exp := "%s %s blah %s"

	if str, _, err := formatGetVerbs(reg); err != nil {
		t.Error(err)
	} else if str != exp {
		fmt.Printf("%s\n", str)
		t.Error("Unexpected parsed verbs")
	}
}

func TestFormatRegularString(t *testing.T) {
	reg := "Regular string"

	if str, err := formatPhpString(reg, ""); err != nil {
		t.Error(err)
	} else {
		if str != reg {
			t.Error("Unexpected regular string")
		}
	}
}

func TestFormatPhpString(t *testing.T) {
	phpstr := `a:4:{i:0;s:21:"Legal compliance Docs";i:1;s:15:"tx_dam_cat:9930";i:2;s:5:"Media";i:3;s:1:"1";}`
	fmtstr := `Record '%s' (%s) was inserted on page '%s' (%s)`
	result := `Record 'Legal compliance Docs' (tx_dam_cat:9930) was inserted on page 'Media' (1)`

	if str, err := formatPhpString(fmtstr, phpstr); err != nil {
		t.Error(err)
	} else {
		if str != result {
			fmt.Printf("%s\n", str)
			t.Error("Unexpected PHP formatted string")
		}
	}
}
