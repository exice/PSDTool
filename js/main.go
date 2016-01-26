//go:generate gopherjs build -m -o psdtool.js

package main

import (
	"bytes"
	"fmt"
	"image/color"
	"io"
	"log"
	"time"

	"github.com/gopherjs/gopherjs/js"
	"github.com/oov/psd"
)

type root struct {
	Width  int
	Height int
	Layer  []layer
}

type layer struct {
	Name                  string
	BlendMode             string
	Opacity               uint8
	Clipping              bool
	BlendClippedElements  bool
	TransparencyProtected bool
	Visible               bool
	X                     int
	Y                     int
	Width                 int
	Height                int
	Folder                bool
	FolderOpen            bool
	Canvas                *js.Object
	Layer                 []layer
	psdLayer              *psd.Layer
}

func main() {
	js.Global.Set("parsePSD", parsePSD)
}

func arrayBufferToByteSlice(a *js.Object) []byte {
	return js.Global.Get("Uint8Array").New(a).Interface().([]byte)
}

func buildLayer(l *layer) error {
	var err error

	l.Name = l.psdLayer.Name
	l.BlendMode = l.psdLayer.BlendMode.String()
	l.Opacity = l.psdLayer.Opacity
	l.Clipping = l.psdLayer.Clipping
	l.BlendClippedElements = l.psdLayer.BlendClippedElements
	l.Visible = l.psdLayer.Visible()
	l.X = l.psdLayer.Rect.Min.X
	l.Y = l.psdLayer.Rect.Min.Y
	l.Width = l.psdLayer.Rect.Dx()
	l.Height = l.psdLayer.Rect.Dy()
	l.Folder = l.psdLayer.Folder()
	l.FolderOpen = l.psdLayer.FolderIsOpen()

	if l.psdLayer.HasImage() && l.psdLayer.Rect.Dx()*l.psdLayer.Rect.Dy() > 0 {
		if l.Canvas, err = createCanvas(l.psdLayer); err != nil {
			return err
		}
	}
	for i := range l.psdLayer.Layer {
		l.Layer = append(l.Layer, layer{psdLayer: &l.psdLayer.Layer[i]})
		if err = buildLayer(&l.Layer[i]); err != nil {
			return err
		}
	}
	return nil
}

func createCanvas(l *psd.Layer) (*js.Object, error) {
	if l.Picker.ColorModel() != color.NRGBAModel {
		return nil, fmt.Errorf("Unsupported color mode")
	}

	w, h := l.Rect.Dx(), l.Rect.Dy()
	cvs := js.Global.Get("document").Call("createElement", "canvas")
	cvs.Set("width", w)
	cvs.Set("height", h)
	ctx := cvs.Call("getContext", "2d")
	imgData := ctx.Call("createImageData", w, h)
	data := imgData.Get("data")

	var ofsd, ofss, x, y, sx, dx int
	r, g, b := l.Channel[0], l.Channel[1], l.Channel[2]
	rp, gp, bp := r.Data, g.Data, b.Data
	if a, ok := l.Channel[-1]; ok {
		ap := a.Data
		for y = 0; y < h; y++ {
			ofss = y * w
			ofsd = ofss << 2
			for x = 0; x < w; x++ {
				sx, dx = ofss+x, ofsd+x<<2
				data.SetIndex(dx+0, rp[sx])
				data.SetIndex(dx+1, gp[sx])
				data.SetIndex(dx+2, bp[sx])
				data.SetIndex(dx+3, ap[sx])
			}
		}
	} else {
		for y = 0; y < h; y++ {
			ofss = y * w
			ofsd = ofss << 2
			for x = 0; x < w; x++ {
				sx, dx = ofss+x, ofsd+x<<2
				data.SetIndex(dx+0, rp[sx])
				data.SetIndex(dx+1, gp[sx])
				data.SetIndex(dx+2, bp[sx])
				data.SetIndex(dx+3, 0xff)
			}
		}
	}
	ctx.Call("putImageData", imgData, 0, 0)
	return cvs, nil
}

func parse(r io.Reader) (*root, error) {
	s := time.Now().UnixNano()
	psdImg, _, err := psd.Decode(r)
	if err != nil {
		return nil, err
	}
	e := time.Now().UnixNano()
	log.Println("Decode PSD Structure:", (e-s)/1e6)

	if psdImg.Config.ColorMode != psd.ColorModeRGB {
		return nil, fmt.Errorf("Unsupported color mode")
	}

	s = time.Now().UnixNano()
	var l root
	l.Width = psdImg.Config.Rect.Dx()
	l.Height = psdImg.Config.Rect.Dy()
	for i := range psdImg.Layer {
		l.Layer = append(l.Layer, layer{psdLayer: &psdImg.Layer[i]})
		buildLayer(&l.Layer[i])
	}
	e = time.Now().UnixNano()
	log.Println("Build Canvas:", (e-s)/1e6)
	return &l, nil
}

func parsePSD(in *js.Object) *root {
	root, err := parse(bytes.NewBuffer(arrayBufferToByteSlice(in)))
	if err != nil {
		panic(err)
	}
	return root
}
