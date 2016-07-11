package main

import (
	"fmt"

	"github.com/polaris1119/bitmap"
)

var (
	bitSize     = uint64(bitmap.BitmapSize - 1)
	appnetIPMap *IPMap
)

type IPMap struct {
	*bitmap.Bitmap
}

//指定范围内没有设置位的最低索引
func (this *IPMap) FindLowestUnsetBit(start uint64, end uint64) (uint64, error) {
	if start > bitSize || end > bitSize || start > end {
		return uint64(0), fmt.Errorf("[%v:%v] out of bitmap range", start, end)
	}

	for i := start; i <= end; i++ {
		if this.GetBit(i) == uint8(0) {
			return i, nil
		}
	}

	return uint64(0), fmt.Errorf("all bit set")

}

//获取最大没有设置的位
func (this *IPMap) FindUnsetBitAfterMaxpos() uint64 {
	return this.Maxpos() + 1
}

func init() {

	bmap := bitmap.NewBitmap()
	appnetIPMap = &IPMap{
		bmap,
	}

}
