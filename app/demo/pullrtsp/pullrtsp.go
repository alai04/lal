// Copyright 2020, Chef.  All rights reserved.
// https://github.com/q191201771/lal
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/q191201771/lal/pkg/base"
	"github.com/q191201771/lal/pkg/httpflv"
	"github.com/q191201771/lal/pkg/remux"
	"github.com/q191201771/lal/pkg/rtsp"
	"github.com/q191201771/naza/pkg/nazalog"
)

func main() {
	_ = nazalog.Init(func(option *nazalog.Option) {
		option.AssertBehavior = nazalog.AssertFatal
	})
	defer nazalog.Sync()

	inURL, outFilename, overTCP := parseFlag()

	var fileWriter httpflv.FLVFileWriter
	err := fileWriter.Open(outFilename)
	nazalog.Assert(nil, err)
	defer fileWriter.Dispose()
	err = fileWriter.WriteRaw(httpflv.FLVHeader)
	nazalog.Assert(nil, err)

	remuxer := remux.NewAVPacket2RTMPRemuxer(func(msg base.RTMPMsg) {
		err = fileWriter.WriteTag(*remux.RTMPMsg2FLVTag(msg))
		nazalog.Assert(nil, err)
	})
	pullSession := rtsp.NewPullSession(remuxer, func(option *rtsp.PullSessionOption) {
		option.PullTimeoutMS = 5000
		option.OverTCP = overTCP != 0
	})

	err = pullSession.Pull(inURL)
	nazalog.Assert(nil, err)
	defer pullSession.Dispose()

	go func() {
		for {
			pullSession.UpdateStat(1)
			nazalog.Debugf("stat. pull=%+v", pullSession.GetStat())
			time.Sleep(1 * time.Second)
		}
	}()

	err = <-pullSession.WaitChan()
	nazalog.Infof("< pullSession.Wait(). err=%+v", err)
}

func parseFlag() (inURL string, outFilename string, overTCP int) {
	i := flag.String("i", "", "specify pull rtsp url")
	o := flag.String("o", "", "specify ouput flv file")
	t := flag.Int("t", 0, "specify interleaved mode(rtp/rtcp over tcp)")
	flag.Parse()
	if *i == "" || *o == "" {
		flag.Usage()
		_, _ = fmt.Fprintf(os.Stderr, `Example:
  %s -i rtsp://localhost:5544/live/test110 -o out.flv -t 0
  %s -i rtsp://localhost:5544/live/test110 -o out.flv -t 1
`, os.Args[0], os.Args[0])
		base.OSExitAndWaitPressIfWindows(1)
	}
	return *i, *o, *t
}
