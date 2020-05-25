package process

import (
	"sync"

	"github.com/pion/ion/pkg/log"
	"github.com/pion/ion/pkg/node/avp/elements"
	"github.com/pion/ion/pkg/node/avp/process/samplebuilder"
	"github.com/pion/ion/pkg/rtc/transport"
)

var (
	config Config
)

// Config for pipeline
type Config struct {
	SampleBuilder samplebuilder.Config
	WebmSaver     elements.WebmSaverConfig
}

// Pipeline constructs a processing graph
//
//                                         +--->node
//                                         |
// pub--->pubCh-->sampleBuilder-->nodeCh---+--->node
//                                         |
//                                         +--->node
type Pipeline struct {
	pub           transport.Transport
	elements      map[string]elements.Element
	elementLock   sync.RWMutex
	elementChans  map[string]chan *samplebuilder.Sample
	sampleBuilder *samplebuilder.SampleBuilder
	stop          bool
}

// InitPipeline .
func InitPipeline(c Config) {
	config = c

	elements.InitWebmSaver(c.WebmSaver)
}

// NewPipeline return a new Pipeline
func NewPipeline(id string, pub transport.Transport) *Pipeline {
	log.Infof("NewPipeline id=%s", id)
	p := &Pipeline{
		pub:           pub,
		elements:      make(map[string]elements.Element),
		elementChans:  make(map[string]chan *samplebuilder.Sample),
		sampleBuilder: samplebuilder.NewSampleBuilder(config.SampleBuilder),
	}

	if config.WebmSaver.DefaultOn {
		webm := elements.NewWebmSaver(id)
		p.AddElement(elements.TypeWebmSaver, webm)
	}

	p.start()

	return p
}

func (p *Pipeline) start() {
	go func() {
		for {
			if p.stop {
				return
			}
			pkt, err := p.pub.ReadRTP()
			if err != nil {
				log.Errorf("p.pub.ReadRTP err=%v", err)
				continue
			}
			p.sampleBuilder.WriteRTP(pkt)
		}
	}()

	go func() {
		for {
			if p.stop {
				return
			}

			sample := p.sampleBuilder.Read()

			p.elementLock.RLock()
			// Push to client send queues
			for _, element := range p.elements {
				element.Write(sample)
			}
			p.elementLock.RUnlock()
		}
	}()
}

// AddElement add a element to pipeline
func (p *Pipeline) AddElement(name string, e elements.Element) {
	if p.elements[name] != nil {
		log.Errorf("Pipeline.AddElement element %s already exists.", name)
		return
	}
	p.elementLock.Lock()
	defer p.elementLock.Unlock()
	p.elements[name] = e
	p.elementChans[name] = make(chan *samplebuilder.Sample, 100)
	log.Infof("Pipeline.AddElement name=%s", name)
}

// GetElement get a node by id
func (p *Pipeline) GetElement(id string) elements.Element {
	p.elementLock.RLock()
	defer p.elementLock.RUnlock()
	return p.elements[id]
}

// DelElement del node by id
func (p *Pipeline) DelElement(id string) {
	log.Infof("Pipeline.DelElement id=%s", id)
	p.elementLock.Lock()
	defer p.elementLock.Unlock()
	if p.elements[id] != nil {
		p.elements[id].Close()
	}
	if p.elementChans[id] != nil {
		close(p.elementChans[id])
	}
	delete(p.elements, id)
	delete(p.elementChans, id)
}

func (p *Pipeline) delElements() {
	p.elementLock.RLock()
	keys := make([]string, 0, len(p.elements))
	for k := range p.elements {
		keys = append(keys, k)
	}
	p.elementLock.RUnlock()

	for _, id := range keys {
		p.DelElement(id)
	}
}

func (p *Pipeline) delPub() {
	if p.pub != nil {
		p.pub.Close()
	}
	p.sampleBuilder.Stop()
	p.pub = nil
}

// Close release all
func (p *Pipeline) Close() {
	if p.stop {
		return
	}
	log.Infof("Pipeline.Close")
	p.delPub()
	p.stop = true
	p.delElements()
}
