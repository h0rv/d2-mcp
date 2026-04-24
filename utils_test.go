package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestFindSVGToPNGConverterRequiresRSVG(t *testing.T) {
	converter, err := findSVGToPNGConverterWithLookPath(func(name string) (string, error) {
		if name == "rsvg-convert" {
			return "/usr/bin/rsvg-convert", nil
		}
		return "", errors.New("not found")
	})
	if err != nil {
		t.Fatalf("find converter: %v", err)
	}
	if converter.name != "rsvg-convert" {
		t.Fatalf("converter = %q, want rsvg-convert", converter.name)
	}
	wantArgs := []string{"-f", "png", "-o", "out.png", "in.svg"}
	if got := converter.args("in.svg", "out.png"); !reflect.DeepEqual(got, wantArgs) {
		t.Fatalf("args = %#v, want %#v", got, wantArgs)
	}
}

func TestFindSVGToPNGConverterErrorsWithoutConverter(t *testing.T) {
	_, err := findSVGToPNGConverterWithLookPath(func(string) (string, error) {
		return "", errors.New("not found")
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
