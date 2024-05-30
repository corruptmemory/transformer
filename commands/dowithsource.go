package commands

import (
	"io"
	"os"

	fsutils "transformer/util"
)

func DoWithSources[T any](c *Commands, contextName string, errorNotFound error, sources []string, do func(string) (T, error)) (result T, err error) {
	for _, source := range sources {
		c.Debugf("%s: checking source: %s", contextName, source)
		e, f, _, err := fsutils.WhatsThere(source)
		if err != nil {
			c.Debugf("%s: error checking source: %v", contextName, err)
			continue
		}
		if e && f {
			file, err := os.Open(source)
			if err != nil {
				c.Debugf("%s: error opening source: %v", contextName, err)
				continue
			}
			defer file.Close()
			bts, err := io.ReadAll(file)
			if err != nil {
				c.Debugf("%s: error reading source: %v", contextName, err)
				continue
			}
			result, err = do(string(bts))
			if err != nil {
				c.Debugf("%s: error parsing source: %v", contextName, err)
				continue
			}
			return result, nil
		}
	}
	c.Errorf("%s: no source found", contextName)
	return result, errorNotFound
}

func DoWithSourcesWithErrorProcessor[T any](
	c *Commands,
	contextName string,
	errorNotFound error,
	sources []string,
	do func(string) (T, error),
	examineError func(in error) (shouldReturn bool),
) (result T, err error) {
	for _, source := range sources {
		c.Debugf("%s: checking source: %s", contextName, source)
		e, f, _, err := fsutils.WhatsThere(source)
		if err != nil {
			c.Debugf("%s: error checking source: %v", contextName, err)
			continue
		}
		if e && f {
			file, err := os.Open(source)
			if err != nil {
				c.Debugf("%s: error opening source: %v", contextName, err)
				continue
			}
			defer file.Close()
			bts, err := io.ReadAll(file)
			if err != nil {
				c.Debugf("%s: error reading source: %v", contextName, err)
				continue
			}
			result, err = do(string(bts))
			if err != nil {
				shouldReturn := examineError(err)
				if shouldReturn {
					c.Errorf("%s: error parsing source: %v", contextName, err)
					return result, err
				}
				c.Debugf("%s: error parsing source: %v", contextName, err)
				continue
			}
			return result, nil
		}
	}
	c.Error("%s: no source found", contextName)
	return result, errorNotFound
}
