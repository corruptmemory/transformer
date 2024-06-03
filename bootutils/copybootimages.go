package bootutils

import (
	"regexp"
)

var (
	configRe  = regexp.MustCompile(`^config-\S+$`)
	initrdRe  = regexp.MustCompile(`^initrd.img-\S+$`)
	mapRe     = regexp.MustCompile(`^System.map-\S+$`)
	vmlinuzRe = regexp.MustCompile(`^vmlinuz-\S+$`)
)

func IsBootFile(in string) bool {
	if configRe.MatchString(in) {
		return true
	}
	if initrdRe.MatchString(in) {
		return true
	}
	if mapRe.MatchString(in) {
		return true
	}
	if vmlinuzRe.MatchString(in) {
		return true
	}
	return false
}
