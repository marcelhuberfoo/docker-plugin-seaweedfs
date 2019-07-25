package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/sirupsen/logrus"
)

const socketAddress = "/run/docker/plugins/seaweedfs.sock"

type seaweedfsVolume struct {
	Options []string

	Name, Mountpoint string
	connections      int
}

type seaweedfsDriver struct {
	sync.RWMutex

	root      string
	statePath string
	volumes   map[string]*seaweedfsVolume
}

func newseaweedfsDriver(root string) (*seaweedfsDriver, error) {
	logrus.WithField("method", "new driver").Debug(root)

	d := &seaweedfsDriver{
		root:      filepath.Join(root, "volumes"),
		statePath: filepath.Join(root, "state", "seaweedfs-state.json"),
		volumes:   map[string]*seaweedfsVolume{},
	}

	data, err := ioutil.ReadFile(d.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			logrus.WithField("statePath", d.statePath).Debug("no state found")
		} else {
			return nil, err
		}
	} else {
		if err := json.Unmarshal(data, &d.volumes); err != nil {
			return nil, err
		}
	}

	return d, nil
}

func (d *seaweedfsDriver) saveState() {
	data, err := json.Marshal(d.volumes)
	if err != nil {
		logrus.WithField("statePath", d.statePath).Error(err)
		return
	}

	if err := ioutil.WriteFile(d.statePath, data, 0644); err != nil {
		logrus.WithField("savestate", d.statePath).Error(err)
	}
}

// Create Instructs the plugin that the user wants to create a volume,
// given a user specified volume name. The plugin does not need to actually
// manifest the volume on the filesystem yet (until Mount is called).
// Opts is a map of driver specific options passed through from the user request.
func (d *seaweedfsDriver) Create(r *volume.CreateRequest) error {
	logrus.WithField("method", "create").Debugf("%#v", r)

	d.Lock()
	defer d.Unlock()
	v := &seaweedfsVolume{}

	for key, val := range r.Options {
		switch key {
		default:
			if val != "" {
				v.Options = append(v.Options, key+"="+val)
			} else {
				v.Options = append(v.Options, key)
			}
		}
	}

	// if v.Sshcmd == "" {
	// 	return logError("'sshcmd' option required")
	// }
	//v.Mountpoint = filepath.Join(d.root, fmt.Sprintf("%x", md5.Sum([]byte(v.Sshcmd))))
	v.Mountpoint = filepath.Join("/mnt/docker-volumes", r.Name)
	v.Name = r.Name

	d.volumes[r.Name] = v

	d.saveState()

	return nil
}

// Remove the specified volume from disk. This request is issued when a
// user invokes docker rm -v to remove volumes associated with a container.
func (d *seaweedfsDriver) Remove(r *volume.RemoveRequest) error {
	logrus.WithField("method", "remove").Debugf("%#v", r)

	d.Lock()
	defer d.Unlock()

	v, ok := d.volumes[r.Name]
	if !ok {
		return logError("volume %s not found", r.Name)
	}

	if v.connections != 0 {
		return logError("volume %s is currently used by a container", r.Name)
	}
	if err := os.RemoveAll(v.Mountpoint); err != nil {
		return logError(err.Error())
	}
	delete(d.volumes, r.Name)
	d.saveState()
	return nil
}

// Path requests the path to the volume with the given volume_name.
func (d *seaweedfsDriver) Path(r *volume.PathRequest) (*volume.PathResponse, error) {
	logrus.WithField("method", "path").Debugf("%#v", r)

	d.RLock()
	defer d.RUnlock()

	v, ok := d.volumes[r.Name]
	if !ok {
		return &volume.PathResponse{}, logError("volume %s not found", r.Name)
	}

	return &volume.PathResponse{Mountpoint: v.Mountpoint}, nil
}

// Mount is called once per container start.
// If the same volume_name is requested more than once, the plugin may need to keep
// track of each new mount request and provision at the first mount request and
// deprovision at the last corresponding unmount request.
// Docker requires the plugin to provide a volume, given a user specified volume name.
// ID is a unique ID for the caller that is requesting the mount.
func (d *seaweedfsDriver) Mount(r *volume.MountRequest) (*volume.MountResponse, error) {
	logrus.WithField("method", "mount").Debugf("%#v", r)

	d.Lock()
	defer d.Unlock()

	v, ok := d.volumes[r.Name]
	if !ok {
		return &volume.MountResponse{}, logError("volume %s not found", r.Name)
	}

	if v.connections == 0 {
		fi, err := os.Lstat(v.Mountpoint)
		if os.IsNotExist(err) {
			if err := os.MkdirAll(v.Mountpoint, 0755); err != nil {
				return &volume.MountResponse{}, logError(err.Error())
			}
		} else if err != nil {
			return &volume.MountResponse{}, logError(err.Error())
		}

		if fi != nil && !fi.IsDir() {
			return &volume.MountResponse{}, logError("%v already exist and it's not a directory", v.Mountpoint)
		}

		if err := d.mountVolume(v); err != nil {
			return &volume.MountResponse{}, logError(err.Error())
		}
	}

	v.connections++

	return &volume.MountResponse{Mountpoint: v.Mountpoint}, nil
}

// Docker is no longer using the named volume.
// Unmount is called once per container stop.
// Plugin may deduce that it is safe to deprovision the volume at this point.
// ID is a unique ID for the caller that is requesting the mount.
func (d *seaweedfsDriver) Unmount(r *volume.UnmountRequest) error {
	logrus.WithField("method", "unmount").Debugf("%#v", r)

	d.Lock()
	defer d.Unlock()
	v, ok := d.volumes[r.Name]
	if !ok {
		return logError("volume %s not found", r.Name)
	}

	v.connections--

	if v.connections <= 0 {
		//TODO: need to remove the 		"--name=seaweed-volume-plugin-"+v.Name, container

		v.connections = 0
	}

	return nil
}

// Get info about volume_name.
func (d *seaweedfsDriver) Get(r *volume.GetRequest) (*volume.GetResponse, error) {
	logrus.WithField("method", "get").Debugf("%#v", r)

	d.Lock()
	defer d.Unlock()

	v, ok := d.volumes[r.Name]
	if !ok {
		return &volume.GetResponse{}, logError("volume %s not found", r.Name)
	}

	return &volume.GetResponse{Volume: &volume.Volume{Name: r.Name, Mountpoint: v.Mountpoint}}, nil
}

// List of volumes registered with the plugin.
func (d *seaweedfsDriver) List() (*volume.ListResponse, error) {
	logrus.WithField("method", "list").Debugf("")

	d.Lock()
	defer d.Unlock()

	var vols []*volume.Volume
	for name, v := range d.volumes {
		vols = append(vols, &volume.Volume{Name: name, Mountpoint: v.Mountpoint})
	}
	return &volume.ListResponse{Volumes: vols}, nil
}

// Get the list of capabilities the driver supports.
// The driver is not required to implement Capabilities. If it is not implemented, the default values are used.
func (d *seaweedfsDriver) Capabilities() *volume.CapabilitiesResponse {
	logrus.WithField("method", "capabilities").Debugf("")

	return &volume.CapabilitiesResponse{Capabilities: volume.Capability{Scope: "local"}}
}

func (d *seaweedfsDriver) mountVolume(v *seaweedfsVolume) error {
	cmd := exec.Command(
		"docker",
		"run",
		"--rm",
		"-d",
		"--name=seaweed-volume-plugin-"+v.Name,
		"--net=seaweedfs_internal",
		"--mount", "type=bind,src="+getPluginDir()+"/propagated-mount/,dst=/mnt/docker-volumes/,bind-propagation=rshared",
		"--cap-add=SYS_ADMIN",
		"--device=/dev/fuse:/dev/fuse",
		"--security-opt=apparmor:unconfined",
		"--entrypoint=weed",
		"svendowideit/seaweedfs-volume-plugin-rootfs:next", // TODO: need to figure this out dynamically
		"mount",
		"-filer=filer:8888",
		"-dir="+v.Mountpoint,
		"-filer.path="+v.Mountpoint,
	)

	// for _, option := range v.Options {
	// 	cmd.Args = append(cmd.Args, "-o", option)
	// }

	logrus.Debug(cmd.Args)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return logError("seaweedfs command execute failed: %v (%s)", err, output)
	}
	return nil
}

func getPluginDir() string {
	// TODO: store this, but verify - as it could change any time... (and if it does, that's a signal to re-run the mount)
	// write a unique filename to /tmp
	content := []byte("temporary file's content")
	tmpfile, err := ioutil.TempFile("/tmp", "example")
	if err != nil {
		log.Fatal(err)
	}

	defer os.Remove(tmpfile.Name()) // clean up

	if _, err := tmpfile.Write(content); err != nil {
		log.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		log.Fatal(err)
	}
	// start a container with access to /var/lib/docker/plugins/ and search for that file in */rootfs/tmp
	filename := strings.TrimPrefix(tmpfile.Name(), "/tmp/")
	cmd := exec.Command(
		"docker",
		"run",
		"--rm",
		"-v=/var/lib/docker/plugins/:/var/lib/docker/plugins/",
		"--entrypoint=find",
		"svendowideit/seaweedfs-volume-plugin-rootfs:next", // TODO: need to figure this out dynamically
		"/var/lib/docker/plugins/",
		"-name", filename,
	)
	// use it.
	logrus.Debug(cmd.Args)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Debugf("seaweedfs command execute failed: %v (%s)", err, output)
		return ""
	}
	pluginDir := strings.TrimSpace(string(output))
	pluginDir = strings.TrimSuffix(pluginDir, "/rootfs"+tmpfile.Name())
	logrus.Debug(pluginDir)
	return pluginDir
}

func logError(format string, args ...interface{}) error {
	logrus.Errorf(format, args...)
	return fmt.Errorf(format, args...)
}

func main() {
	debug := os.Getenv("DEBUG")
	if ok, _ := strconv.ParseBool(debug); ok {
		logrus.SetLevel(logrus.DebugLevel)
	}

	d, err := newseaweedfsDriver("/mnt")
	if err != nil {
		log.Fatal(err)
	}
	h := volume.NewHandler(d)
	logrus.Infof("listening on %s", socketAddress)
	logrus.Error(h.ServeUnix(socketAddress, 0))
}
