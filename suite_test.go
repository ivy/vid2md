package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestVid2md(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "vid2md")
}
