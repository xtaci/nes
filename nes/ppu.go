package nes

import "log"

type PPU struct {
	Memory      // memory interface
	nes    *NES // reference to parent object

	Cycle         int    // 0-340
	ScanLine      int    // 0-261, 0-239=visible, 240=post, 241-260=vblank, 261=pre
	Frame         uint64 // frame counter
	VerticalBlank byte   // vertical blank status

	// $2000 PPUCTRL
	flagNametable       byte // 0: $2000; 1: $2400; 2: $2800; 3: $2C00
	flagIncrement       byte // 0: add 1; 1: add 32
	flagSpriteTable     byte // 0: $0000; 1: $1000; ignored in 8x16 mode
	flagBackgroundTable byte // 0: $0000; 1: $1000
	flagSpriteSize      byte // 0: 8x8; 1: 8x16
	flagMasterSlave     byte // 0: read EXT; 1: write EXT
	flagGenerateNMI     byte // 0: off; 1: on

	// $2001 PPUMASK
	flagGrayscale          byte // 0: color; 1: grayscale
	flagShowLeftBackground byte // 0: hide; 1: show
	flagShowLeftSprites    byte // 0: hide; 1: show
	flagShowBackground     byte // 0: hide; 1: show
	flagShowSprites        byte // 0: hide; 1: show
	flagRedTint            byte // 0: normal; 1: emphasized
	flagGreenTint          byte // 0: normal; 1: emphasized
	flagBlueTint           byte // 0: normal; 1: emphasized

	// $2003 OAMADDR
	oamAddress byte

	// $2004 OAMDATA
	oamData [256]byte

	// $2005 PPUSCROLL
	scroll uint16 // x & y scrolling coordinates

	// $2006 PPUADDR
	address uint16 // address used by $2007 PPUDATA

	paletteData   [32]byte
	nametableData [2048]byte
}

func NewPPU(nes *NES) *PPU {
	ppu := PPU{Memory: nes.PPUMemory, nes: nes}
	ppu.Reset()
	return &ppu
}

func (ppu *PPU) Reset() {
	ppu.Cycle = 340
	ppu.ScanLine = 240
	ppu.Frame = 0
	ppu.VerticalBlank = 0
	ppu.writeControl(0)
	ppu.writeMask(0)
	ppu.writeOAMAddress(0)
}

func (ppu *PPU) ReadRegister(address uint16) byte {
	switch address {
	case 0x2002:
		return ppu.readStatus()
	case 0x2004:
		return ppu.readOAMData()
	case 0x2007:
		return ppu.readData()
	default:
		log.Fatalf("unhandled ppu register read at address: 0x%04X", address)
	}
	return 0
}

func (ppu *PPU) WriteRegister(address uint16, value byte) {
	switch address {
	case 0x2000:
		ppu.writeControl(value)
	case 0x2001:
		ppu.writeMask(value)
	case 0x2003:
		ppu.writeOAMAddress(value)
	case 0x2004:
		ppu.writeOAMData(value)
	case 0x2005:
		ppu.writeScroll(value)
	case 0x2006:
		ppu.writeAddress(value)
	case 0x2007:
		ppu.writeData(value)
	case 0x4014:
		ppu.writeDMA(value)
	default:
		log.Fatalf("unhandled ppu register write at address: 0x%04X", address)
	}
}

// $2000: PPUCTRL
func (ppu *PPU) writeControl(value byte) {
	ppu.flagNametable = (value >> 0) & 3
	ppu.flagIncrement = (value >> 2) & 1
	ppu.flagSpriteTable = (value >> 3) & 1
	ppu.flagBackgroundTable = (value >> 4) & 1
	ppu.flagSpriteSize = (value >> 5) & 1
	ppu.flagMasterSlave = (value >> 6) & 1
	ppu.flagGenerateNMI = (value >> 7) & 1
}

// $2001: PPUMASK
func (ppu *PPU) writeMask(value byte) {
	ppu.flagGrayscale = (value >> 0) & 1
	ppu.flagShowLeftBackground = (value >> 1) & 1
	ppu.flagShowLeftSprites = (value >> 2) & 1
	ppu.flagShowBackground = (value >> 3) & 1
	ppu.flagShowSprites = (value >> 4) & 1
	ppu.flagRedTint = (value >> 5) & 1
	ppu.flagGreenTint = (value >> 6) & 1
	ppu.flagBlueTint = (value >> 7) & 1
}

// $2002: PPUSTATUS
func (ppu *PPU) readStatus() byte {
	var result byte
	result |= ppu.VerticalBlank << 7
	ppu.VerticalBlank = 0
	return result
}

// $2003: OAMADDR
func (ppu *PPU) writeOAMAddress(value byte) {
	ppu.oamAddress = value
}

// $2004: OAMDATA (read)
func (ppu *PPU) readOAMData() byte {
	return ppu.oamData[ppu.oamAddress]
}

// $2004: OAMDATA (write)
func (ppu *PPU) writeOAMData(value byte) {
	ppu.oamData[ppu.oamAddress] = value
	ppu.oamAddress++
}

// $2005: PPUSCROLL
func (ppu *PPU) writeScroll(value byte) {
	ppu.scroll <<= 8
	ppu.scroll |= uint16(value)
}

// $2006: PPUADDR
func (ppu *PPU) writeAddress(value byte) {
	ppu.address <<= 8
	ppu.address |= uint16(value)
}

// $2007: PPUDATA (read)
func (ppu *PPU) readData() byte {
	value := ppu.Read(ppu.address)
	if ppu.flagIncrement == 0 {
		ppu.address += 1
	} else {
		ppu.address += 32
	}
	return value
}

// $2007: PPUDATA (write)
func (ppu *PPU) writeData(value byte) {
	ppu.Write(ppu.address, value)
	if ppu.flagIncrement == 0 {
		ppu.address += 1
	} else {
		ppu.address += 32
	}
}

// $4014: OAMDMA
func (ppu *PPU) writeDMA(value byte) {
	// TODO: stall CPU for 513 or 514 cycles
	cpu := ppu.nes.CPU
	address := uint16(value) << 8
	for i := 0; i < 256; i++ {
		ppu.oamData[ppu.oamAddress] = cpu.Read(address)
		ppu.oamAddress++
		address++
	}
}

// tick updates Cycle, ScanLine and Frame counters
func (ppu *PPU) tick() {
	ppu.Cycle++
	if ppu.Cycle > 340 {
		ppu.Cycle = 0
		ppu.ScanLine++
		if ppu.ScanLine > 261 {
			ppu.ScanLine = 0
			ppu.Frame++
		}
	}
}

// Step executes a single PPU cycle
func (ppu *PPU) Step() {
	ppu.tick()
	if ppu.ScanLine == 241 && ppu.Cycle == 1 {
		ppu.VerticalBlank = 1
	}
	if ppu.ScanLine == 261 && ppu.Cycle == 1 {
		ppu.VerticalBlank = 0
	}
}