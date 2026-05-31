package solana

import "fmt"

const b58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

var b58Decode [128]int8

func init() {
	for i := range b58Decode {
		b58Decode[i] = -1
	}
	for i, c := range b58Alphabet {
		b58Decode[c] = int8(i)
	}
}

func Base58Encode(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	zeros := 0
	for zeros < len(b) && b[zeros] == 0 {
		zeros++
	}
	size := len(b)*138/100 + 1
	buf := make([]byte, size)
	high := size - 1
	for _, x := range b {
		carry := uint32(x)
		i := size - 1
		for ; i > high || carry != 0; i-- {
			carry += uint32(buf[i]) * 256
			buf[i] = byte(carry % 58)
			carry /= 58
		}
		high = i
	}
	j := 0
	for j < size && buf[j] == 0 {
		j++
	}
	out := make([]byte, zeros+size-j)
	for i := 0; i < zeros; i++ {
		out[i] = b58Alphabet[0]
	}
	for k, x := range buf[j:] {
		out[zeros+k] = b58Alphabet[x]
	}
	return string(out)
}

func Base58Decode(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	zeros := 0
	for zeros < len(s) && s[zeros] == b58Alphabet[0] {
		zeros++
	}
	size := len(s)*733/1000 + 1
	buf := make([]byte, size)
	high := size - 1
	for _, c := range []byte(s) {
		if c >= 128 || b58Decode[c] == -1 {
			return nil, fmt.Errorf("invalid base58 char %q", c)
		}
		carry := uint32(b58Decode[c])
		i := size - 1
		for ; i > high || carry != 0; i-- {
			carry += uint32(buf[i]) * 58
			buf[i] = byte(carry % 256)
			carry /= 256
		}
		high = i
	}
	j := 0
	for j < size && buf[j] == 0 {
		j++
	}
	out := make([]byte, zeros+size-j)
	copy(out[zeros:], buf[j:])
	return out, nil
}
