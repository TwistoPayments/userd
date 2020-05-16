package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
)

func ensureFileContent(filename string, content []byte) (err error) {
	existingContent, err := ioutil.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			existingContent = []byte{}
		} else {
			return err
		}
	}
	if bytes.Equal(existingContent, content) { // Nothing to do
		return nil
	}

	tf, err := ioutil.TempFile(filepath.Dir(filename), filepath.Base(filename))
	if err != nil {
		return err
	}

	_, err = tf.Write(content)
	if err == nil {
		_, err = tf.Write([]byte("\n"))
	}
	if err != nil {
		_ = os.Remove(tf.Name())
		return err
	}

	err = tf.Close()
	if err != nil {
		_ = os.Remove(tf.Name())
		return err
	}

	err = os.Rename(tf.Name(), filename)
	return err
}
