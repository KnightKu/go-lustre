package fs

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.intel.com/hpdd/lustre"
	"github.intel.com/hpdd/lustre/llapi"
	"github.intel.com/hpdd/lustre/status"
)

// Version returns the current Lustre version string.
func Version() string {
	v, err := llapi.GetVersion()
	if err != nil {
		log.Printf("GetVersion failed: %v\n", err)
		return ""
	}
	return v
}

// MountID returns the local Lustre client indentifier for that mountpoint. This can
// be used to determine which entries in /proc/fs/lustre as associated with
// that client.
func MountID(mountPath string) (*status.LustreClient, error) {
	id, err := llapi.GetName(mountPath)
	if err != nil {
		return nil, err
	}
	elem := strings.Split(id, "-")
	c := status.LustreClient{FsName: elem[0], ClientID: elem[1]}
	return &c, nil
}

// RootDir represent a the mount point of a Lustre filesystem.
type RootDir string

type mountDir struct {
	path   RootDir
	lock   sync.Mutex
	opened bool
	f      *os.File
}

// A cache of file handles per lustre mount point. Currently used to fetch the host Mdt for a file.
// Could merge with RootDir and ensure RootDir is a singleton per client
var openMount map[RootDir]*mountDir

func init() {
	openMount = make(map[RootDir]*mountDir)
}

func (m *mountDir) open() error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if !m.opened {
		f, err := os.Open(string(m.path))
		if err != nil {
			return err
		}

		m.f = f
		m.opened = true
	}
	return nil
}

func (m *mountDir) String() string {
	return string(m.path)
}

func (m *mountDir) GetMdt(in *lustre.Fid) (int, error) {
	if !m.opened {
		err := m.open()
		if err != nil {
			return 0, err
		}
	}
	mdtIndex, err := llapi.GetMdtIndexByFid(int(m.f.Fd()), in)
	if err != nil {
		return 0, err
	}
	return mdtIndex, nil
}

func getOpenMount(root RootDir) *mountDir {
	//	var mnt *mountDir
	mnt, ok := openMount[root]
	if !ok {
		mnt = &mountDir{path: root}
		openMount[root] = mnt
	}
	return mnt
}

// GetMdt returns the MDT index for a given Fid
func GetMdt(root RootDir, f *lustre.Fid) (int, error) {
	mnt := getOpenMount(root)
	return mnt.GetMdt(f)
}

// Join args with root dir to create an absolute path.
// FIXME: replace this with OpenAt and friends
func (root RootDir) Join(args ...string) string {
	return path.Join(string(root), path.Join(args...))
}

func (root RootDir) String() string {
	return string(root)
}

// Path returns the path for the root
func (root RootDir) Path() string {
	return string(root)
}

// ID should be a unique identifier for a filesystem. For now just use RootDir
type ID RootDir

func (root ID) String() string {
	return string(root)
}

// Path returns the path for the root
func (root ID) Path() (string, error) {
	return string(root), nil
}

// GetID returns the filesystem's ID. For the moment, this is the root path, but in
// the future it could be something more globally unique (uuid?).
func GetID(p string) (ID, error) {
	r, err := MountRoot(p)
	if err != nil {
		return ID(r), err
	}
	return ID(r), nil
}

// Determine if given directory is the one true magical DOT_LUSTRE directory.
func isDotLustre(dir string) bool {
	fi, err := os.Lstat(dir)
	if err != nil {
		return false
	}
	if fi.IsDir() {
		fid, err := LookupFid(dir)
		if err == nil && fid.IsDotLustre() {
			return true
		}
	}
	return false
}

// Return root device from the struct stat embedded in FileInfo
func rootDevice(fi os.FileInfo) uint64 {
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if ok {
		return stat.Dev
	}
	panic("no stat available")
}

// findRoot returns the root directory for the lustre filesystem containing
// the pathname. If the the filesystem is not lustre, then error is returned.
func findRoot(dev uint64, pathname string) string {
	parent := path.Dir(pathname)
	fi, err := os.Lstat(parent)
	if err != nil {
		return ""
	}
	//  If "/" is lustre then we won't see the device change
	if rootDevice(fi) != dev || pathname == "/" {
		if isDotLustre(path.Join(pathname, ".lustre")) {
			return pathname
		}
		return ""
	}

	return findRoot(dev, parent)
}

// MountRoot returns the Lustre filesystem mountpoint for the give path
// or returns an error if the path is not on a Lustre filesystem.
func MountRoot(path string) (RootDir, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return RootDir(""), err
	}

	mnt := findRoot(rootDevice(fi), path)
	if mnt == "" {
		return RootDir(""), fmt.Errorf("%s not a Lustre filesystem", path)
	}
	return RootDir(mnt), nil
}

// findRelPah returns pathname relative to root directory for the lustre filesystem containing
// the pathname. If no Lustre root was found, then empty strings are returned.
func findRelPath(dev uint64, pathname string, relPath []string) (string, string) {
	parent := path.Dir(pathname)
	fi, err := os.Lstat(parent)
	if err != nil {
		return "", ""
	}
	//  If "/" is lustre then we won't see the device change
	if rootDevice(fi) != dev || pathname == "/" {
		if isDotLustre(path.Join(pathname, ".lustre")) {
			return pathname, path.Join(relPath...)
		}
		return "", ""
	}

	return findRelPath(dev, parent, append([]string{path.Base(pathname)}, relPath...))
}

// MountRelPath returns the lustre mountpoint, and remaing path for the given pathname. The remaining  paht
// is relative to the mount point. Returns an error if pathname is not valid or does not refer to a Lustre fs.
func MountRelPath(pathname string) (RootDir, string, error) {
	pathname = filepath.Clean(pathname)
	fi, err := os.Lstat(pathname)
	if err != nil {
		return RootDir(""), "", err
	}

	root, relPath := findRelPath(rootDevice(fi), pathname, []string{})
	if root == "" {
		return RootDir(""), "", fmt.Errorf("%s not a Lustre filesystem", pathname)
	}
	return RootDir(root), relPath, nil
}