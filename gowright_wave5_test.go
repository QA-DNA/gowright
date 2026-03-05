package gowright_test

import (
	"testing"

	"github.com/PeterStoica/gowright"
)

func TestDownloadType(t *testing.T) {
	t.Parallel()
	var d gowright.Download
	_ = d.URL()
	_ = d.SuggestedFilename()
}

func TestFileChooserType(t *testing.T) {
	t.Parallel()
	var fc gowright.FileChooser
	_ = fc.IsMultiple()
}
