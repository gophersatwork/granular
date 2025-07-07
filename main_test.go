package granular

import (
	"os"
	"testing"
	"time"
)

func TestMain(t *testing.M) {
	code := t.Run()

	os.Exit(code)
}

func fixedNowFunc() time.Time {
	return time.Date(2020, 3, 1, 0, 0, 0, 0, time.UTC)
}