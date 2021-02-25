package image

import (
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"k8s.io/klog/v2"
	"strings"
)

// WriteImageToDaemon write img to the local docker daemon
func WriteImageToDaemon(img string) error {
	klog.Infof("Writing %s to local daemon", img)
	ref, err := name.ParseReference(img)
	if err != nil {
		return errors.Wrap(err, "parsing reference")
	}
	klog.V(3).Infof("Getting image %v", ref)
	i, err := remote.Image(ref)
	if err != nil {
		if strings.Contains(err.Error(), "GitHub Docker Registry needs login") {
			ErrGithubNeedsLogin = errors.New(err.Error())
			return ErrGithubNeedsLogin
		} else if strings.Contains(err.Error(), "UNAUTHORIZED") {
			ErrNeedsLogin = errors.New(err.Error())
			return ErrNeedsLogin
		}

		return errors.Wrap(err, "getting remote image")
	}
	klog.V(3).Infof("Writing image %v", ref)

	_, err = daemonWrite(ref, i)
	if err != nil {
		return errors.Wrap(err, "writing daemon image")
	}
	return nil
}

func daemonWrite(ref name.Reference, img v1.Image) (string, error) {
	cli, err := daemon.GetImageLoader()
	if err != nil {
		return "", err
	}

	pr, pw := io.Pipe()
	go func() {
		pw.CloseWithError(tarball.Write(ref, img, pw))
	}()

	// write the image in docker save format first, then load it
	resp, err := cli.ImageLoad(context.Background(), pr, false)
	if err != nil {
		return "", fmt.Errorf("error loading image: %v", err)
	}
	defer resp.Body.Close()
	b, readErr := ioutil.ReadAll(resp.Body)
	response := string(b)
	if readErr != nil {
		return response, fmt.Errorf("error reading load response body: %v", err)
	}
	return response, nil
}
