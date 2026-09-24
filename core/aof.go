package core

import (
	"fmt"
	"log"
	"os"
	"redis_internals/config"
	"strings"
)

func dumpKey(fp *os.File, key string, obj *Obj) {
	cmd := fmt.Sprintf("SET %s %s", key, obj.Value)
	tokens := strings.Split(cmd, " ")
	fp.Write(Encode(tokens, false))
}
func DumpAllAOF() {
	fp, err := os.OpenFile(config.AOFFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0777)
	if err != nil {
		fmt.Print("error", err)
		return
	}

	log.Println("rewriting AOF file at", config.AOFFile)

	for k, obj := range store {
		dumpKey(fp, k, obj)
	}

	log.Println("AOF file rewrite complete")
}
