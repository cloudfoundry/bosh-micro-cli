package job

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	biinstallblob "github.com/cloudfoundry/bosh-init/installation/blob"
	biui "github.com/cloudfoundry/bosh-init/ui"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type RenderedJobRef struct {
	Name        string
	Version     string
	BlobstoreID string
	SHA1        string
}

type InstalledJob struct {
	RenderedJobRef
	Path string
}

func NewInstalledJob(ref RenderedJobRef, path string) InstalledJob {
	return InstalledJob{RenderedJobRef: ref, Path: path}
}

type Installer interface {
	Install(RenderedJobRef, biui.Stage) (InstalledJob, error)
	Cleanup(InstalledJob) error
}

func NewInstaller(
	fs boshsys.FileSystem,
	templateExtractor biinstallblob.Extractor,
	jobsPath string,
) Installer {
	return jobInstaller{
		fs:                fs,
		templateExtractor: templateExtractor,
		jobsPath:          jobsPath,
	}
}

type jobInstaller struct {
	fs                boshsys.FileSystem
	templateExtractor biinstallblob.Extractor
	jobsPath          string
}

func (i jobInstaller) Install(renderedJobRef RenderedJobRef, stage biui.Stage) (installedJob InstalledJob, err error) {
	stageName := fmt.Sprintf("Installing job '%s'", renderedJobRef.Name)
	err = stage.Perform(stageName, func() error {
		installedJob, err = i.install(renderedJobRef)
		return err
	})
	return installedJob, err
}

func (i jobInstaller) install(renderedJobRef RenderedJobRef) (InstalledJob, error) {
	jobDir := filepath.Join(i.jobsPath, renderedJobRef.Name)
	err := i.fs.MkdirAll(jobDir, os.ModePerm)
	if err != nil {
		return InstalledJob{}, bosherr.WrapErrorf(err, "Creating job directory '%s'", jobDir)
	}

	err = i.templateExtractor.Extract(renderedJobRef.BlobstoreID, renderedJobRef.SHA1, jobDir)
	if err != nil {
		return InstalledJob{}, bosherr.WrapErrorf(err, "Extracting blob with ID '%s'", renderedJobRef.BlobstoreID)
	}

	binFiles := path.Join(jobDir, "bin", "*")
	files, err := i.fs.Glob(binFiles)
	if err != nil {
		return InstalledJob{}, bosherr.WrapErrorf(err, "Globbing %s", binFiles)
	}

	for _, file := range files {
		err = i.fs.Chmod(file, os.FileMode(0755))
		if err != nil {
			return InstalledJob{}, bosherr.WrapErrorf(err, "Making %s executable", binFiles)
		}
	}

	return NewInstalledJob(renderedJobRef, jobDir), nil
}

func (i jobInstaller) Cleanup(job InstalledJob) error {
	err := i.fs.RemoveAll(job.Path)

	if err != nil {
		return bosherr.WrapErrorf(err, "Removing job installed to %s", job.Path)
	}

	return nil
}