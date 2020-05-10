package plugins

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pion/rtp"

	"github.com/pion/ion/pkg/log"
)

// RTPWriterConfig .
type RTPWriterConfig struct {
	ID     string
	OutDir string
	On     bool
}

// RTPWriter core
type RTPWriter struct {
	id         string
	fo         *os.File
	outRTPChan chan *rtp.Packet
}

// NewRTPWriter Create new RTP Writer
func NewRTPWriter(config RTPWriterConfig) *RTPWriter {
	log.Infof("New RTPWriter Plugin with id %s path %s", config.ID, config.OutDir)
	fo, err := os.Create(filepath.Join(config.OutDir, fmt.Sprintf("%s.ionrtp", config.ID)))
	if err != nil {
		panic(err)
	}
	return &RTPWriter{
		id:         config.ID,
		fo:         fo,
		outRTPChan: make(chan *rtp.Packet, maxSize),
	}
}

// ID Return RTPWriter ID
func (w *RTPWriter) ID() string {
	return w.id
}

// WriteRTP Forward rtp packet from pub
func (w *RTPWriter) WriteRTP(pkt *rtp.Packet) error {
	w.outRTPChan <- pkt
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	enc.Encode(pkt)
	_, err := w.fo.Write(buf.Bytes())
	return err
}

// ReadRTP Forward rtp packet from pub
func (w *RTPWriter) ReadRTP() <-chan *rtp.Packet {
	return w.outRTPChan
}

// Stop Stop plugin
func (w *RTPWriter) Stop() {
	w.fo.Close()
}
