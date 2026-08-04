// Copyright (c) 2015-2026 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cmd

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/minio/madmin-go/v3"
)

// Paths that must never be accepted from an internode payload. Each is a
// distinct evasion of the segment scanner: forward slash, backslash (Windows
// resolves these, path.Clean does not), whitespace padding, and single-dot.
var traversalPaths = []string{
	"../evil",
	"../../evil",
	"a/../../evil",
	"..\\evil",
	"a\\..\\..\\evil",
	" .. /evil",
	"../",
	"./../evil",
}

// Paths that alias the volume root itself: pathJoin collapses each of these
// back to the volume directory, so renaming or deleting one relocates or
// destroys the whole volume, with no ".." required.
//
// Only the platform-independent forms live here. Whitespace does NOT belong:
// path.Clean leaves spaces alone, so "  " names a real directory inside the
// volume and is a legal S3 object key. Backslash forms do not belong either:
// they collapse on Windows but name an ordinary file on Unix. Both classes are
// covered by TestIsVolumeRootAliasIsPlatformCorrect and, from the other
// direction, by TestGuardAcceptsEveryLegalObjectName.
var volumeRootAliases = []string{"", "/", "//"}

// sentinels plants files we can prove were neither read nor removed.
type sentinels struct {
	drive     string // drive root
	outside   string // file outside the drive root entirely
	sibling   string // file in a sibling volume
	legit     string // a legitimate object inside "foo"
	partDir   string // a readable part dir in the sibling volume
	outsideRe string // path a rename/write would land on, outside the drive
}

func plantSentinels(t *testing.T, drive string) *sentinels {
	t.Helper()
	s := &sentinels{
		drive:     drive,
		outside:   filepath.Join(filepath.Dir(drive), "sentinel-outside.txt"),
		sibling:   filepath.Join(drive, "bar", "sentinel-sibling.txt"),
		legit:     filepath.Join(drive, "foo", "legit.txt"),
		partDir:   filepath.Join(drive, "bar", "obj"),
		outsideRe: filepath.Join(filepath.Dir(drive), "sentinel-landing.txt"),
	}
	mustWrite(t, s.outside, "OUTSIDE-SECRET")
	t.Cleanup(func() { os.Remove(s.outside); os.Remove(s.outsideRe) })
	mustWrite(t, s.sibling, "SIBLING-SECRET")
	mustWrite(t, s.legit, "LEGIT")

	if err := os.MkdirAll(s.partDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(s.partDir, "part.1"), "data")
	blob, err := (&ObjectPartInfo{Number: 1, Size: 4, ETag: "SIBLING-ETAG"}).MarshalMsg(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.partDir, "part.1.meta"), blob, 0o644); err != nil {
		t.Fatal(err)
	}
	return s
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// assertIntact fails if any sentinel was removed, modified, or if a file
// appeared where a traversal would have landed.
func (s *sentinels) assertIntact(t *testing.T, op string) {
	t.Helper()
	for _, f := range []struct{ path, want string }{
		{s.outside, "OUTSIDE-SECRET"},
		{s.sibling, "SIBLING-SECRET"},
		{s.legit, "LEGIT"},
	} {
		got, err := os.ReadFile(f.path)
		if err != nil {
			t.Errorf("%s: sentinel %s was destroyed: %v", op, f.path, err)
			continue
		}
		if string(got) != f.want {
			t.Errorf("%s: sentinel %s was modified: got %q want %q", op, f.path, got, f.want)
		}
	}
	if _, err := os.Stat(s.outsideRe); err == nil {
		t.Errorf("%s: a file was created outside the drive root at %s", op, s.outsideRe)
	}
	for _, vol := range []string{"foo", "bar"} {
		st, err := os.Stat(filepath.Join(s.drive, vol))
		if err != nil || !st.IsDir() {
			t.Errorf("%s: volume %q no longer exists as a directory (err=%v)", op, vol, err)
		}
	}
}

// assertDenied requires the specific sentinel error, not merely "some error".
// An operation on a nonexistent path fails anyway, so a bare err != nil check
// passes even when the guard is absent.
func assertDenied(t *testing.T, op string, err error) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: expected %v, got nil", op, errFileAccessDenied)
		return
	}
	if !errors.Is(err, errFileAccessDenied) {
		t.Errorf("%s: expected %v, got %v (%T)", op, errFileAccessDenied, err, err)
	}
}

// TestStorageRESTTraversalRejected drives the real REST/grid client against a
// real xlStorage and proves that no internode payload can read, write, move or
// delete anything outside its own volume.
//
// NOTE: the grid.SetupTestGrid harness mounts a bare mux router with none of
// globalMiddlewares, so a green run says nothing about the production HTTP
// query-argument path (which setRequestValidityMiddleware already covers). It
// measures the storage-layer guards and nothing else, which is the point.
func TestStorageRESTTraversalRejected(t *testing.T) {
	restClient := newStorageRESTHTTPServerClient(t)
	drive := globalLocalSetDrives[0][0][0].Endpoint().Path
	s := plantSentinels(t, drive)
	ctx := t.Context()

	badFI := func(dataDir string) FileInfo {
		return FileInfo{Volume: "foo", Name: "obj", DataDir: dataDir, ModTime: UTCNow()}
	}

	// Every operation that accepts a path from the wire, driven with each
	// traversal form. The op must be denied AND must not have touched disk.
	for _, p := range traversalPaths {
		ops := []struct {
			name string
			run  func() error
		}{
			{"ReadAll", func() error { _, err := restClient.ReadAll(ctx, "foo", p); return err }},
			{"WriteAll", func() error { return restClient.WriteAll(ctx, "foo", p, []byte("PWNED")) }},
			{"ReadXL", func() error { _, err := restClient.ReadXL(ctx, "foo", p, true); return err }},
			{"Delete", func() error {
				return restClient.Delete(ctx, "foo", p, DeleteOptions{Recursive: true, Immediate: true})
			}},
			{"DeleteBulk", func() error { return restClient.DeleteBulk(ctx, "foo", p) }},
			{"ReadParts", func() error { _, err := restClient.ReadParts(ctx, "foo", p); return err }},
			{"StatInfoFile", func() error { _, err := restClient.StatInfoFile(ctx, "foo", p, false); return err }},
			{"CleanAbandonedData", func() error { return restClient.CleanAbandonedData(ctx, "foo", p) }},
			{"ListDir", func() error { _, err := restClient.ListDir(ctx, "", "foo", p, -1); return err }},
			{"RenameFile-src", func() error { return restClient.RenameFile(ctx, "foo", p, "foo", "dst.txt") }},
			{"RenameFile-dst", func() error { return restClient.RenameFile(ctx, "foo", "legit.txt", "foo", p) }},
			{"RenamePart-src", func() error {
				return restClient.RenamePart(ctx, "foo", p, "foo", "dst.txt", nil, "")
			}},
			{"RenamePart-dst", func() error {
				return restClient.RenamePart(ctx, "foo", "legit.txt", "foo", p, nil, "")
			}},
			{"RenamePart-skipParent", func() error {
				return restClient.RenamePart(ctx, "foo", "legit.txt", "foo", "dst.txt", nil, p)
			}},
			{"CheckParts", func() error { _, err := restClient.CheckParts(ctx, "foo", p, badFI("")); return err }},
			{"CheckParts-DataDir", func() error {
				_, err := restClient.CheckParts(ctx, "foo", "obj", badFI(p))
				return err
			}},
			{"VerifyFile", func() error { _, err := restClient.VerifyFile(ctx, "foo", p, badFI("")); return err }},
			{"VerifyFile-DataDir", func() error {
				_, err := restClient.VerifyFile(ctx, "foo", "obj", badFI(p))
				return err
			}},
			{"WriteMetadata", func() error { return restClient.WriteMetadata(ctx, "", "foo", p, badFI("")) }},
			{"UpdateMetadata", func() error {
				return restClient.UpdateMetadata(ctx, "foo", p, badFI(""), UpdateMetadataOpts{})
			}},
			{"DeleteVersion", func() error {
				return restClient.DeleteVersion(ctx, "foo", p, badFI(""), false, DeleteOptions{})
			}},

			// Nested path-bearing fields, poisoned one at a time. A method that
			// validates its `path` argument but forgets one of these still
			// passes every check above, so each needs its own case.
			{"WriteMetadata-DataDir", func() error {
				return restClient.WriteMetadata(ctx, "", "foo", "obj", badFI(p))
			}},
			{"UpdateMetadata-DataDir", func() error {
				return restClient.UpdateMetadata(ctx, "foo", "obj", badFI(p), UpdateMetadataOpts{})
			}},
			{"DeleteVersion-DataDir", func() error {
				return restClient.DeleteVersion(ctx, "foo", "obj", badFI(p), false, DeleteOptions{})
			}},
			// DeleteOptions.OldDataDir is guarded too, but cannot be exercised
			// from here: DeleteVersionHandler hardcodes `opts := DeleteOptions{}`
			// and never reads the wire value, so the field is unreachable today.
			// That is an accidental mitigation, not a control - see
			// TestGuardChecksUnreachableFields for the unit-level assertion that
			// the guard is ready if anyone ever plumbs it through.
			{"RenameData-srcPath", func() error {
				_, err := restClient.RenameData(ctx, "foo", p, badFI(""), "bar", "dst", RenameOptions{})
				return err
			}},
			{"RenameData-dstPath", func() error {
				_, err := restClient.RenameData(ctx, "foo", "src", badFI(""), "bar", p, RenameOptions{})
				return err
			}},
			{"RenameData-DataDir", func() error {
				_, err := restClient.RenameData(ctx, "foo", "src", badFI(p), "bar", "dst", RenameOptions{})
				return err
			}},
		}
		for _, op := range ops {
			t.Run(op.name+"/"+p, func(t *testing.T) {
				assertDenied(t, op.name, op.run())
				s.assertIntact(t, op.name)
			})
		}

		// The nested DataDir of each version is a separate field from the
		// per-object Name; poison them independently so neither can regress
		// behind the other.
		t.Run("DeleteVersions-VersionDataDir/"+p, func(t *testing.T) {
			errs := restClient.DeleteVersions(ctx, "foo",
				[]FileInfoVersions{{Name: "obj", Versions: []FileInfo{{Name: "obj", DataDir: p}}}},
				DeleteOptions{})
			if len(errs) != 1 {
				t.Fatalf("expected 1 error, got %d", len(errs))
			}
			assertDenied(t, "DeleteVersions-VersionDataDir", errs[0])
			s.assertIntact(t, "DeleteVersions-VersionDataDir")
		})

		t.Run("DeleteVersions/"+p, func(t *testing.T) {
			errs := restClient.DeleteVersions(ctx, "foo",
				[]FileInfoVersions{{Name: p, Versions: []FileInfo{{Name: p}}}}, DeleteOptions{})
			if len(errs) != 1 {
				t.Fatalf("expected 1 error, got %d", len(errs))
			}
			assertDenied(t, "DeleteVersions", errs[0])
			s.assertIntact(t, "DeleteVersions")
		})

		// WalkDir and NSScanner take their paths inside an options/cache struct
		// rather than as arguments, which is exactly where a per-handler check
		// tends to miss them.
		t.Run("WalkDir/"+p, func(t *testing.T) {
			for _, opts := range []WalkDirOptions{
				{Bucket: p, BaseDir: "obj"},
				{Bucket: "foo", BaseDir: p},
			} {
				if err := restClient.WalkDir(ctx, opts, io.Discard); err == nil {
					t.Errorf("WalkDir(%+v) returned nil", opts)
				}
			}
			s.assertIntact(t, "WalkDir")
		})

		t.Run("NSScanner/"+p, func(t *testing.T) {
			cache := dataUsageCache{Info: dataUsageCacheInfo{Name: p}}
			updates := make(chan dataUsageEntry, 1)
			if _, err := restClient.NSScanner(ctx, cache, updates, madmin.HealNormalScan, nil); err == nil {
				t.Errorf("NSScanner(cache.Info.Name=%q) returned nil", p)
			}
			s.assertIntact(t, "NSScanner")
		})

		t.Run("ReadAll-volume/"+p, func(t *testing.T) {
			// Traversal smuggled through the volume argument rather than the path.
			buf, err := restClient.ReadAll(ctx, p, "sentinel-outside.txt")
			if err == nil {
				t.Errorf("ReadAll with volume %q: expected error, got nil (read %d bytes)", p, len(buf))
			}
			if string(buf) == "OUTSIDE-SECRET" {
				t.Errorf("ReadAll with volume %q leaked a file outside the drive root", p)
			}
			s.assertIntact(t, "ReadAll-volume")
		})
	}

	// Volume-root aliases: no ".." involved, but a rename or bulk delete of the
	// volume root relocates or destroys the entire volume.
	for _, p := range volumeRootAliases {
		t.Run("DeleteBulk-root/"+p, func(t *testing.T) {
			assertDenied(t, "DeleteBulk", restClient.DeleteBulk(ctx, "foo", p))
			s.assertIntact(t, "DeleteBulk-root")
		})
		t.Run("RenameFile-root/"+p, func(t *testing.T) {
			assertDenied(t, "RenameFile", restClient.RenameFile(ctx, "foo", p, "bar", "captured"))
			s.assertIntact(t, "RenameFile-root")
		})
	}

	// A batch mixing a legitimate target with a malicious one must delete
	// neither -- validate the whole set before acting on any of it.
	t.Run("MixedBatch", func(t *testing.T) {
		assertDenied(t, "DeleteBulk-mixed",
			restClient.DeleteBulk(ctx, "foo", "legit.txt", "../bar/sentinel-sibling.txt"))
		s.assertIntact(t, "DeleteBulk-mixed")
	})

	t.Run("ReadParts-mixed", func(t *testing.T) {
		_, err := restClient.ReadParts(ctx, "foo", "legit.txt", "../bar/obj/part.1.meta")
		assertDenied(t, "ReadParts-mixed", err)
		s.assertIntact(t, "ReadParts-mixed")
	})
}

// TestStorageRESTLegitimateOpsStillWork is the positive control: the guards must
// not reject anything the cluster does in normal operation.
func TestStorageRESTLegitimateOpsStillWork(t *testing.T) {
	restClient := newStorageRESTHTTPServerClient(t)
	drive := globalLocalSetDrives[0][0][0].Endpoint().Path
	ctx := t.Context()

	mustWrite(t, filepath.Join(drive, "foo", "a.txt"), "HELLO")
	mustWrite(t, filepath.Join(drive, minioMetaBucket, "tmp", "sys.txt"), "SYS")

	if err := restClient.WriteAll(ctx, "foo", "written.txt", []byte("DATA")); err != nil {
		t.Fatalf("WriteAll on a legitimate path: %v", err)
	}
	if b, err := restClient.ReadAll(ctx, "foo", "written.txt"); err != nil || string(b) != "DATA" {
		t.Fatalf("ReadAll on a legitimate path: %q %v", b, err)
	}
	// Reserved volumes contain dot-prefixed segments (".minio.sys", ".trash")
	// which must remain acceptable -- only exact "." and ".." segments are bad.
	if b, err := restClient.ReadAll(ctx, minioMetaBucket, "tmp/sys.txt"); err != nil || string(b) != "SYS" {
		t.Fatalf("ReadAll on %s: %q %v", minioMetaBucket, b, err)
	}
	if err := restClient.RenameFile(ctx, "foo", "a.txt", "foo", "b.txt"); err != nil {
		t.Fatalf("RenameFile on legitimate paths: %v", err)
	}
	if _, err := restClient.ListDir(ctx, "", "foo", "", -1); err != nil {
		t.Fatalf("ListDir on the volume root is legitimate: %v", err)
	}
	if err := restClient.DeleteBulk(ctx, "foo", "b.txt"); err != nil {
		t.Fatalf("DeleteBulk on a legitimate path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(drive, "foo", "b.txt")); err == nil {
		t.Fatal("DeleteBulk on a legitimate path did not delete")
	}

	// Object keys that look like separators or path components but are not.
	// A whitespace-only key is legal in S3 and is committed through the rename
	// path on PutObject, so a guard that refuses it fails the write on every
	// remote drive at once and breaks quorum. This is a real regression that
	// shipped in an earlier draft of the guard.
	oddKeys := []string{" ", "  ", "\t", "a b", " lead", "trail ", "..foo", "foo..", "a/..b"}
	if runtime.GOOS != globalWindowsOSName {
		// Backslash is an ordinary filename character off Windows.
		oddKeys = append(oddKeys, "\\", "\\\\", "a\\b", "/\\")
	}
	for _, key := range oddKeys {
		if !IsValidObjectName(key) {
			t.Fatalf("test bug: %q is not a legal object name", key)
		}
		if err := restClient.WriteAll(ctx, "foo", key, []byte("ODD")); err != nil {
			t.Errorf("WriteAll(%q) on a legal object key: %v", key, err)
			continue
		}
		if b, err := restClient.ReadAll(ctx, "foo", key); err != nil || string(b) != "ODD" {
			t.Errorf("ReadAll(%q) on a legal object key: %q %v", key, b, err)
		}
		// The rename path is what PutObject uses to commit.
		if err := restClient.RenameFile(ctx, "foo", key, "foo", key+"-renamed"); err != nil {
			t.Errorf("RenameFile(%q) on a legal object key: %v", key, err)
			continue
		}
		if err := restClient.DeleteBulk(ctx, "foo", key+"-renamed"); err != nil {
			t.Errorf("DeleteBulk(%q) on a legal object key: %v", key+"-renamed", err)
		}
	}
}

// TestPeerS3VolumeTraversalRejected covers the peer-S3 bucket RPCs, which reach
// local drives through globalLocalDrivesMap and never pass through
// storageRESTServer.getStorage(). Only the getVolDir guard protects them.
func TestPeerS3VolumeTraversalRejected(t *testing.T) {
	newStorageRESTHTTPServerClient(t)
	drive := globalLocalSetDrives[0][0][0].Endpoint().Path
	ctx := t.Context()

	victim := filepath.Join(filepath.Dir(drive), "peer-victim")
	mustWrite(t, filepath.Join(victim, "nested", "important.txt"), "IMPORTANT")
	t.Cleanup(func() { os.RemoveAll(victim) })

	for _, p := range traversalPaths {
		t.Run("DeleteBucket/"+p, func(t *testing.T) {
			// Force:true reaches moveToTrash(volumeDir, recursive, immediate).
			if err := deleteBucketLocal(ctx, p+"/peer-victim", DeleteBucketOptions{Force: true}); err == nil {
				t.Errorf("deleteBucketLocal(%q) returned nil", p)
			}
			if _, err := os.Stat(filepath.Join(victim, "nested", "important.txt")); err != nil {
				t.Errorf("deleteBucketLocal(%q) destroyed a tree outside the drive root: %v", p, err)
			}
		})
		t.Run("MakeBucket/"+p, func(t *testing.T) {
			created := filepath.Join(filepath.Dir(drive), "peer-created")
			t.Cleanup(func() { os.RemoveAll(created) })
			if err := makeBucketLocal(ctx, p+"/peer-created", MakeBucketOptions{}); err == nil {
				t.Errorf("makeBucketLocal(%q) returned nil", p)
			}
			if _, err := os.Stat(created); err == nil {
				t.Errorf("makeBucketLocal(%q) created a directory outside the drive root", p)
			}
		})
	}
}

// TestCheckPartsMalformedErasure covers a remote node kill that is not a
// traversal: CheckParts hands a wire-supplied FileInfo to ShardFileSize, which
// divided by ErasureInfo.BlockSize with no validity check. The call runs inside
// xioutil.WithDeadline, i.e. a bare goroutine, so the resulting integer
// divide-by-zero panic cannot be recovered by the grid or net/http handlers --
// it terminates the process. If this test regresses it does not fail, it
// crashes the whole run.
func TestCheckPartsMalformedErasure(t *testing.T) {
	restClient := newStorageRESTHTTPServerClient(t)
	drive := globalLocalSetDrives[0][0][0].Endpoint().Path
	ctx := t.Context()

	// Not panicking is the floor, not the bar. ShardFileSize returns 0 for these,
	// and checkPart's "st.Size() < expectedSize" then succeeds for *any* file
	// that exists - including a truncated shard - so a request that got this far
	// would come back reporting every part intact. The boundary must refuse it
	// outright, which is what errFileCorrupt asserts here.
	for _, fi := range []FileInfo{
		{Volume: "foo", Name: "obj", Parts: []ObjectPartInfo{{Number: 1, Size: 4}}},
		// Deleted short-circuits FileInfo.IsValid() to true, so an IsValid()
		// guard would not have caught this one.
		{Volume: "foo", Name: "obj", Deleted: true, Parts: []ObjectPartInfo{{Number: 1, Size: 4}}},
		{Volume: "foo", Name: "obj", Parts: []ObjectPartInfo{{Number: 1, Size: 4}},
			Erasure: ErasureInfo{DataBlocks: 4}}, // BlockSize still zero
		{Volume: "foo", Name: "obj", Parts: []ObjectPartInfo{{Number: 1, Size: 4}},
			Erasure: ErasureInfo{BlockSize: blockSizeV2}}, // DataBlocks still zero
	} {
		if _, err := restClient.CheckParts(ctx, "foo", "obj", fi); !errors.Is(err, errFileCorrupt) {
			t.Errorf("CheckParts(%+v): got %v, want %v", fi.Erasure, err, errFileCorrupt)
		}
		if _, err := restClient.VerifyFile(ctx, "foo", "obj", fi); !errors.Is(err, errFileCorrupt) {
			t.Errorf("VerifyFile(%+v): got %v, want %v", fi.Erasure, err, errFileCorrupt)
		}
	}

	// A part of zero length says nothing about the erasure parameters, so it
	// must not be swept up by the rule above.
	zeroPart := FileInfo{Volume: "foo", Name: "obj", Parts: []ObjectPartInfo{{Number: 1, Size: 0}}}
	if _, err := restClient.CheckParts(ctx, "foo", "obj", zeroPart); errors.Is(err, errFileCorrupt) {
		t.Errorf("CheckParts with a zero-length part was rejected as corrupt")
	}

	// A NEGATIVE part size is the sharper case, and the one an ">0" test misses.
	// ShardFileSize returns 0 for it whether or not the erasure parameters are
	// usable (numShards and lastShardSize both floor to zero), so checkPart's
	// "st.Size() < expectedSize" is false for any file that exists and the part
	// is reported healthy. With a real short part planted on disk, a guard that
	// only rejects Size > 0 lets malformed metadata launder a truncated shard
	// into a clean bill of health.
	partDir := filepath.Join(drive, "foo", "negobj")
	if err := os.MkdirAll(partDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(partDir, "part.1"), "tiny")

	for _, e := range []ErasureInfo{
		{}, // unusable parameters
		// Fully valid parameters. This is the sharper case: the FileInfo passes
		// FileInfo.IsValid(), the very check healing uses to decide the metadata
		// is trustworthy, and the negative size still lands on a zero expected
		// shard size. Valid erasure parameters do not save you here.
		{DataBlocks: 2, ParityBlocks: 2, BlockSize: blockSizeV2, Index: 1, Distribution: []int{1, 2, 3, 4}},
	} {
		fi := FileInfo{
			Volume: "foo", Name: "negobj", Erasure: e,
			Parts: []ObjectPartInfo{{Number: 1, Size: -2}},
		}
		if e.DataBlocks > 0 && !fi.IsValid() {
			t.Fatal("test bug: the second case must satisfy FileInfo.IsValid()")
		}
		resp, err := restClient.CheckParts(ctx, "foo", "negobj", fi)
		if !errors.Is(err, errFileCorrupt) {
			t.Errorf("CheckParts with a negative part size (erasure %+v): got err=%v resp=%v, want %v",
				e, err, resp, errFileCorrupt)
			if resp != nil && len(resp.Results) > 0 && resp.Results[0] == checkPartSuccess {
				t.Errorf("  -> and it reported the part HEALTHY, which is the actual damage")
			}
		}
		if _, err := restClient.VerifyFile(ctx, "foo", "negobj", fi); !errors.Is(err, errFileCorrupt) {
			t.Errorf("VerifyFile with a negative part size (erasure %+v): got %v, want %v", e, err, errFileCorrupt)
		}
	}
}

