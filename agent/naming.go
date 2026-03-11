package agent

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/jpoz/taskmaster/wordlist"
)

func GenerateName(dir string, pid int) string {
	key := fmt.Sprintf("%s-%d", dir, pid)
	h := sha256.Sum256([]byte(key))
	idx := binary.BigEndian.Uint16(h[:2]) % uint16(len(wordlist.Words))
	return fmt.Sprintf("%s-%s", dir, wordlist.Words[idx])
}
