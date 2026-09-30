package gt06

import (
	"encoding/binary"
	"fmt"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols"
)

// Delimitadores do GT06. DOCUMENTED: especificação pública GT06/Concox.
const (
	startShort = 0x78 // quadro com 1 byte de tamanho
	startLong  = 0x79 // quadro com 2 bytes de tamanho
	stopCR     = 0x0D
	stopLF     = 0x0A
)

// maxFrameLength protege contra pacotes absurdos antes de alocar (§7).
const maxFrameLength = 4096

// crcITU implementa o CRC-16/X-25 (init 0xFFFF, refletido, xorout 0xFFFF),
// chamado de "CRC-ITU" na documentação do GT06. DOCUMENTED.
func crcITU(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b)
		for range 8 {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0x8408
			} else {
				crc >>= 1
			}
		}
	}
	return ^crc
}

// NextFrame separa um quadro completo do buffer acumulado da conexão.
func (p *Protocol) NextFrame(buf []byte) ([]byte, int, error) {
	start := indexOfStart(buf)
	switch {
	case start < 0:
		// Nenhum delimitador à vista. Mantém o último byte, que pode ser a
		// primeira metade de um delimitador partido entre dois Read().
		keep := 0
		if n := len(buf); n > 0 && (buf[n-1] == startShort || buf[n-1] == startLong) {
			keep = 1
		}
		if len(buf) == keep {
			return nil, 0, protocols.ErrIncompleteFrame
		}
		return nil, len(buf) - keep, fmt.Errorf("%d bytes sem delimitador de início", len(buf)-keep)
	case start > 0:
		return nil, start, fmt.Errorf("%d bytes descartados antes do delimitador", start)
	}

	headerSize := 3 // 2 de início + 1 de tamanho
	if buf[0] == startLong {
		headerSize = 4
	}
	if len(buf) < headerSize {
		return nil, 0, protocols.ErrIncompleteFrame
	}

	var length int
	if buf[0] == startLong {
		length = int(binary.BigEndian.Uint16(buf[2:4]))
	} else {
		length = int(buf[2])
	}

	// length cobre protocolo + conteúdo + serial + CRC.
	if length < 5 || length > maxFrameLength {
		return nil, 2, fmt.Errorf("tamanho de quadro inválido: %d", length)
	}

	total := headerSize + length + 2 // + bytes de parada
	if len(buf) < total {
		return nil, 0, protocols.ErrIncompleteFrame
	}
	if buf[total-2] != stopCR || buf[total-1] != stopLF {
		// Consome apenas o delimitador para ressincronizar no próximo quadro.
		return nil, 2, fmt.Errorf("bytes de parada inesperados: %02X %02X", buf[total-2], buf[total-1])
	}
	return buf[:total], total, nil
}

func indexOfStart(buf []byte) int {
	for i := 0; i+1 < len(buf); i++ {
		if (buf[i] == startShort || buf[i] == startLong) && buf[i+1] == buf[i] {
			return i
		}
	}
	return -1
}

// packet é um quadro já validado.
type packet struct {
	typ     byte
	content []byte
	serial  uint16
}

// decodeFrame confere o CRC e reparte o quadro.
func decodeFrame(frame []byte) (*packet, error) {
	if len(frame) < 9 {
		return nil, fmt.Errorf("quadro curto demais: %d bytes", len(frame))
	}
	headerSize := 3
	if frame[0] == startLong {
		headerSize = 4
	}
	body := frame[headerSize : len(frame)-2]
	if len(body) < 5 {
		return nil, fmt.Errorf("corpo do quadro curto demais: %d bytes", len(body))
	}

	want := binary.BigEndian.Uint16(body[len(body)-2:])
	// O CRC cobre do byte de tamanho até o número de série, inclusive.
	got := crcITU(frame[2 : len(frame)-4])
	if want != got {
		return nil, fmt.Errorf("CRC inválido: quadro traz %04X, calculado %04X", want, got)
	}

	return &packet{
		typ:     body[0],
		content: body[1 : len(body)-4],
		serial:  binary.BigEndian.Uint16(body[len(body)-4 : len(body)-2]),
	}, nil
}

// buildFrame monta 78 78 | tamanho | tipo | conteúdo | serial | CRC | 0D 0A.
func buildFrame(typ byte, content []byte, serial uint16) []byte {
	length := 1 + len(content) + 2 + 2
	out := make([]byte, 0, length+5)
	out = append(out, startShort, startShort, byte(length), typ)
	out = append(out, content...)
	out = binary.BigEndian.AppendUint16(out, serial)
	out = binary.BigEndian.AppendUint16(out, crcITU(out[2:]))
	return append(out, stopCR, stopLF)
}
