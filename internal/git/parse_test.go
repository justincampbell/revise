package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleDiff = "diff --git a/main.go b/main.go\n" +
	"index abc1234..def5678 100644\n" +
	"--- a/main.go\n" +
	"+++ b/main.go\n" +
	"@@ -1,5 +1,6 @@\n" +
	" package main\n" +
	" \n" +
	" import \"fmt\"\n" +
	"+import \"os\"\n" +
	" \n" +
	" func main() {\n" +
	"@@ -10,3 +11,5 @@ func main() {\n" +
	" \tfmt.Println(\"hello\")\n" +
	"+\tfmt.Println(\"world\")\n" +
	"+\tos.Exit(0)\n" +
	" }\n"

func TestParseBasic(t *testing.T) {
	diff := Parse(sampleDiff)

	require.Len(t, diff.Files, 1)
	f := diff.Files[0]
	assert.Equal(t, "main.go", f.Path)
	assert.Equal(t, StatusModified, f.Status)
	require.Len(t, f.Hunks, 2)

	h1 := f.Hunks[0]
	assert.Equal(t, 1, h1.OldStart)
	assert.Equal(t, 5, h1.OldCount)
	assert.Equal(t, 1, h1.NewStart)
	assert.Equal(t, 6, h1.NewCount)

	addedCount := 0
	for _, l := range h1.Lines {
		if l.Type == LineAdded {
			addedCount++
		}
	}
	assert.Equal(t, 1, addedCount, "expected 1 added line in first hunk")
}

const newFileDiff = `diff --git a/new.txt b/new.txt
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/new.txt
@@ -0,0 +1,3 @@
+line one
+line two
+line three
`

func TestParseNewFile(t *testing.T) {
	diff := Parse(newFileDiff)

	require.Len(t, diff.Files, 1)
	f := diff.Files[0]
	assert.Equal(t, StatusAdded, f.Status)
	require.Len(t, f.Hunks, 1)
	assert.Len(t, f.Hunks[0].Lines, 3)
}

const deletedFileDiff = `diff --git a/old.txt b/old.txt
deleted file mode 100644
index abc1234..0000000
--- a/old.txt
+++ /dev/null
@@ -1,2 +0,0 @@
-line one
-line two
`

func TestParseDeletedFile(t *testing.T) {
	diff := Parse(deletedFileDiff)

	require.Len(t, diff.Files, 1)
	assert.Equal(t, StatusDeleted, diff.Files[0].Status)
}

func TestParseEmpty(t *testing.T) {
	diff := Parse("")
	assert.Empty(t, diff.Files)
}

const multiFileDiff = `diff --git a/a.go b/a.go
index abc..def 100644
--- a/a.go
+++ b/a.go
@@ -1,3 +1,4 @@
 package a
+// comment

 func A() {}
diff --git a/b.go b/b.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/b.go
@@ -0,0 +1,3 @@
+package b
+
+func B() {}
`

func TestParseMultipleFiles(t *testing.T) {
	diff := Parse(multiFileDiff)

	require.Len(t, diff.Files, 2)
	assert.Equal(t, "a.go", diff.Files[0].Path)
	assert.Equal(t, "b.go", diff.Files[1].Path)
	assert.Equal(t, StatusAdded, diff.Files[1].Status)
}

const renamedFileDiff = `diff --git a/old.go b/new.go
similarity index 90%
rename from old.go
rename to new.go
index abc1234..def5678 100644
--- a/old.go
+++ b/new.go
@@ -1,3 +1,3 @@
 package main
-// old comment
+// new comment
 func Foo() {}
`

func TestParseRenamedFile(t *testing.T) {
	diff := Parse(renamedFileDiff)

	require.Len(t, diff.Files, 1)
	f := diff.Files[0]
	assert.Equal(t, StatusRenamed, f.Status)
	assert.Equal(t, "old.go", f.OldPath)
	assert.Equal(t, "new.go", f.Path)
}

const binaryFileDiff = `diff --git a/image.png b/image.png
new file mode 100644
index 0000000..abc1234
Binary files /dev/null and b/image.png differ
`

func TestParseBinaryFile(t *testing.T) {
	diff := Parse(binaryFileDiff)

	require.Len(t, diff.Files, 1)
	f := diff.Files[0]
	assert.Equal(t, "image.png", f.Path)
	assert.True(t, f.IsBinary, "expected IsBinary to be true")
	assert.Empty(t, f.Hunks, "binary files should have no hunks")
}

const binaryModifiedDiff = `diff --git a/image.png b/image.png
index abc1234..def5678 100644
Binary files a/image.png and b/image.png differ
`

func TestParseBinaryModifiedFile(t *testing.T) {
	diff := Parse(binaryModifiedDiff)

	require.Len(t, diff.Files, 1)
	f := diff.Files[0]
	assert.Equal(t, "image.png", f.Path)
	assert.True(t, f.IsBinary)
	assert.Equal(t, StatusModified, f.Status)
}

const mixedBinaryAndTextDiff = `diff --git a/image.png b/image.png
new file mode 100644
index 0000000..abc1234
Binary files /dev/null and b/image.png differ
diff --git a/main.go b/main.go
index abc1234..def5678 100644
--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
 package main
+import "os"
 func main() {}
`

func TestParseMixedBinaryAndText(t *testing.T) {
	diff := Parse(mixedBinaryAndTextDiff)

	require.Len(t, diff.Files, 2)
	assert.True(t, diff.Files[0].IsBinary, "image.png should be binary")
	assert.False(t, diff.Files[1].IsBinary, "main.go should not be binary")
	assert.Len(t, diff.Files[1].Hunks, 1)
}

func TestParseLineNumbers(t *testing.T) {
	diff := Parse(sampleDiff)
	h := diff.Files[0].Hunks[0]

	// First line is context: "package main" at old:1, new:1
	assert.Equal(t, 1, h.Lines[0].OldNum)
	assert.Equal(t, 1, h.Lines[0].NewNum)

	// The added line 'import "os"' should be at new:4
	for _, l := range h.Lines {
		if l.Type == LineAdded {
			assert.Equal(t, 4, l.NewNum)
			assert.Equal(t, 0, l.OldNum)
			break
		}
	}
}
