// Licensed to Apache Software Foundation (ASF) under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Apache Software Foundation (ASF) licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package log

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// Default logger config.
const (
	defaultLogPattern  = "%time [%level][%field] - %msg"
	defaultTimePattern = "2006-01-02 15:04:05.000"
)

// LoggerConfig initializes the global logger config.
type LoggerConfig struct {
	LogPattern  string `mapstructure:"log_pattern"`
	TimePattern string `mapstructure:"time_pattern"`
	Level       string `mapstructure:"level"`
}

// FormatOption is a function to set formatter config.
type FormatOption func(f *formatter)

// ConfigOption is a function to set logger config.
type ConfigOption func(l *logrus.Logger)

type formatter struct {
	logPattern  string
	timePattern string
}

// Logger is the global logger.
var Logger *logrus.Logger
var once sync.Once

func Init(cfg *LoggerConfig) {
	once.Do(func() {
		var configOpts []ConfigOption
		var formatOpts []FormatOption

		if cfg.Level != "" {
			configOpts = append(configOpts, SetLevel(cfg.Level))
		} else {
			configOpts = append(configOpts, SetLevel(logrus.InfoLevel.String()))
		}
		if cfg.TimePattern != "" {
			formatOpts = append(formatOpts, SetTimePattern(cfg.TimePattern))
		} else {
			formatOpts = append(formatOpts, SetTimePattern(defaultTimePattern))
		}
		if cfg.LogPattern != "" {
			formatOpts = append(formatOpts, SetLogPattern(cfg.LogPattern))
		} else {
			formatOpts = append(formatOpts, SetLogPattern(defaultLogPattern))
		}
		initBySettings(configOpts, formatOpts)
	})
}

// The Logger init method, keep Logger as a singleton.
func initBySettings(configOpts []ConfigOption, formatOpts []FormatOption) {
	// Default Logger.
	Logger = logrus.New()
	Logger.SetOutput(os.Stdout)
	Logger.SetReportCaller(true) // 启用 caller 信息
	for _, opt := range configOpts {
		opt(Logger)
	}
	// Default formatter.
	f := &formatter{}
	for _, opt := range formatOpts {
		opt(f)
	}
	if !strings.Contains(f.logPattern, "\n") {
		f.logPattern += "\n"
	}
	Logger.SetFormatter(f)
}

// Put the log pattern in formatter.
func SetLogPattern(logPattern string) FormatOption {
	return func(f *formatter) {
		f.logPattern = logPattern
	}
}

// Put the time pattern in formatter.
func SetTimePattern(timePattern string) FormatOption {
	return func(f *formatter) {
		f.timePattern = timePattern
	}
}

// Put the time pattern in formatter.
func SetLevel(levelStr string) ConfigOption {
	return func(logger *logrus.Logger) {
		level, err := logrus.ParseLevel(levelStr)
		if err != nil {
			fmt.Printf("logger level does not exist: %s, level would be set info", levelStr)
			level = logrus.InfoLevel
		}
		logger.SetLevel(level)
	}
}

// Format supports unified log output format that has %time, %level, %field, %msg, %caller, %func, %goroutine.
func (f *formatter) Format(entry *logrus.Entry) ([]byte, error) {
	output := f.logPattern
	output = strings.Replace(output, "%time", entry.Time.Format(f.timePattern), 1)
	output = strings.Replace(output, "%level", entry.Level.String(), 1)
	output = strings.Replace(output, "%field", buildFields(entry), 1)
	output = strings.Replace(output, "%msg", entry.Message, 1)
	output = strings.Replace(output, "%caller", getCaller(entry), 1)
	output = strings.Replace(output, "%func", getFunc(entry), 1)
	output = strings.Replace(output, "%goroutine", getGoroutineID(), 1)
	return []byte(output), nil
}

// 获取调用位置（package/文件名:行号）
func getCaller(entry *logrus.Entry) string {
	if entry.HasCaller() {
		// 只保留 package 和文件名
		file := entry.Caller.File
		// 去除路径，只保留文件名
		slashIdx := strings.LastIndex(file, "/")
		if slashIdx != -1 && slashIdx+1 < len(file) {
			file = file[slashIdx+1:]
		}
		// 获取包名（从函数名中提取）
		pkg := ""
		if entry.Caller.Function != "" {
			funcParts := strings.Split(entry.Caller.Function, ".")
			if len(funcParts) > 1 {
				pkgParts := strings.Split(funcParts[0], "/")
				pkg = pkgParts[len(pkgParts)-1]
			}
		}
		return fmt.Sprintf("%s/%s:%d", pkg, file, entry.Caller.Line)
	}
	// fallback: 使用runtime
	_, file, line, ok := runtime.Caller(8)
	if ok {
		slashIdx := strings.LastIndex(file, "/")
		if slashIdx != -1 && slashIdx+1 < len(file) {
			file = file[slashIdx+1:]
		}
		return fmt.Sprintf("unknown/%s:%d", file, line)
	}
	return "unknown"
}

// 获取调用函数名（只保留方法或函数名）
func getFunc(entry *logrus.Entry) string {
	if entry.HasCaller() {
		funcName := entry.Caller.Function
		// 只保留最后一个点后的部分
		dotIdx := strings.LastIndex(funcName, ".")
		if dotIdx != -1 && dotIdx+1 < len(funcName) {
			return funcName[dotIdx+1:]
		}
		return funcName
	}
	pc, _, _, ok := runtime.Caller(8)
	if ok {
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			funcName := fn.Name()
			dotIdx := strings.LastIndex(funcName, ".")
			if dotIdx != -1 && dotIdx+1 < len(funcName) {
				return funcName[dotIdx+1:]
			}
			return funcName
		}
	}
	return "unknown"
}

// 获取当前 goroutine ID
func getGoroutineID() string {
	// 通过 runtime.Stack hack 获取 goroutine id
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	stack := strings.TrimPrefix(string(buf[:n]), "goroutine ")
	idField := strings.Fields(stack)
	if len(idField) > 0 {
		return idField[0]
	}
	return "unknown"
}

func buildFields(entry *logrus.Entry) string {
	var fields []string
	for key, val := range entry.Data {
		stringVal, ok := val.(string)
		if !ok {
			stringVal = fmt.Sprint(val)
		}
		fields = append(fields, key+"="+stringVal)
	}
	return strings.Join(fields, ",")
}
