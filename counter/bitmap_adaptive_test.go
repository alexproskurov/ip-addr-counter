package counter

import "testing"

func TestAdaptiveBitmapSwitchAndDuplicates(t *testing.T) {
	b := newAdaptiveBitmap()
	if !b.set(0) || b.set(0) {
		t.Fatal("incorrect insertion/duplicate of 0.0.0.0")
	}
	if !b.set(^uint32(0)) || b.set(^uint32(0)) {
		t.Fatal("incorrect insertion/duplicate of 255.255.255.255")
	}
	// Distinct /16 pages trigger dense mode. Include the first/last addresses.
	for i := uint32(1); i < bitsetDenseThreshold; i++ {
		if !b.set(i<<16 | i) {
			t.Fatalf("failed to insert new address on /16 page %d", i)
		}
	}
	if b.dense == nil || b.pages != nil {
		t.Fatal("expected conversion to dense bitset")
	}
	if b.set(0) || b.set(^uint32(0)) {
		t.Fatal("lost bits during conversion")
	}
	for i := uint32(1); i < bitsetDenseThreshold; i++ {
		if b.set(i<<16 | i) {
			t.Fatalf("lost address on /16 page %d", i)
		}
	}
	if !b.set(1<<24|123) || b.set(1<<24|123) {
		t.Fatal("failed to insert new dense-mode bit")
	}
}

func TestAdaptiveBitmapSparseMode(t *testing.T) {
	b := newAdaptiveBitmap()
	for i := uint32(0); i < 30000; i++ {
		if !b.set(0x0a000000 | i) {
			t.Fatalf("missing unique address %d", i)
		}
		if b.set(0x0a000000 | i) {
			t.Fatalf("counted duplicate %d", i)
		}
	}
	if b.dense != nil {
		t.Fatal("dense mode used for clustered IP addresses")
	}
	if b.pageCount != 1 {
		t.Fatalf("page count got %d, want 1", b.pageCount)
	}
}
