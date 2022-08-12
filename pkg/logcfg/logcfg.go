package logcfg

import (
	"fmt"
	"math"
	"strconv"

	log "github.com/sirupsen/logrus"
)

var (
	defaultLogLevel log.Level = log.WarnLevel
)

type PlainFormatter struct {
}

func (f *PlainFormatter) Format(entry *log.Entry) ([]byte, error) {
	return []byte(fmt.Sprintf("%s\n", entry.Message)), nil
}
func ToggleDebug(verbosity string, changed bool) error {
	level, err := parseVerbosity(verbosity, changed)
	if err != nil {
		return err
	}
	log.SetLevel(level)
	return nil
}

func parseVerbosity(verbosity string, changed bool) (log.Level, error) {
	var level log.Level

	if verbosity == "" && changed {
		return log.DebugLevel, nil
	}

	if verbosity == "" && !changed {
		return log.WarnLevel, nil
	}

	numLevel, intErr := strconv.Atoi(verbosity)
	if intErr == nil {
		if numLevel < 0 {
			numLevel = int(math.Abs(float64(numLevel)))
		}
		if numLevel > len(log.AllLevels) {
			numLevel = len(log.AllLevels) - 1
		}
		return log.AllLevels[numLevel], nil
	}

	level, stringErr := log.ParseLevel(verbosity)
	if stringErr == nil {
		return level, nil
	}

	return defaultLogLevel, stringErr
}
