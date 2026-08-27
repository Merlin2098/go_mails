package winpath

import (
	"strings"
	"testing"
)

func TestExtendedLengthPrefix_ShortPathUnchanged(t *testing.T) {
	short := `C:\Boletas\12345678.pdf`
	if got := ExtendedLengthPrefix(short); got != short {
		t.Errorf("expected short path unchanged, got %q", got)
	}
}

func TestExtendedLengthPrefix_LongPathGetsPrefixed(t *testing.T) {
	long := `C:\` + strings.Repeat(`carpeta_muy_larga\`, 20) + `boleta.pdf`
	if len(long) < MaxPath {
		t.Fatalf("test fixture is not actually long enough: %d chars", len(long))
	}

	got := ExtendedLengthPrefix(long)
	if !strings.HasPrefix(got, `\\?\C:\`) {
		t.Errorf("expected extended-length prefix, got %q", got)
	}
}

func TestExtendedLengthPrefix_LongUNCPathGetsUNCPrefix(t *testing.T) {
	long := `\\fileserver\share\` + strings.Repeat(`carpeta_muy_larga\`, 20) + `boleta.pdf`
	if len(long) < MaxPath {
		t.Fatalf("test fixture is not actually long enough: %d chars", len(long))
	}

	got := ExtendedLengthPrefix(long)
	if !strings.HasPrefix(got, `\\?\UNC\fileserver\share\`) {
		t.Errorf("expected UNC extended-length prefix, got %q", got)
	}
}

func TestExtendedLengthPrefix_AlreadyPrefixedUnchanged(t *testing.T) {
	already := `\\?\C:\` + strings.Repeat(`x`, 300) + `.pdf`
	if got := ExtendedLengthPrefix(already); got != already {
		t.Errorf("expected already-prefixed path unchanged, got %q", got)
	}
}

func TestExtendedLengthPrefix_RelativePathUnchanged(t *testing.T) {
	relative := strings.Repeat(`carpeta\`, 40) + `boleta.pdf`
	if len(relative) < MaxPath {
		t.Fatalf("test fixture is not actually long enough: %d chars", len(relative))
	}
	if got := ExtendedLengthPrefix(relative); got != relative {
		t.Errorf("expected relative path unchanged (prefix only applies to absolute paths), got %q", got)
	}
}
