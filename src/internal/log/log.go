package log

import (
	"io/ioutil"
	"log"
	"os"

	"internal/flag"
)

func init() {
	log.SetFlags(0)
	log.SetOutput(ioutil.Discard)

	if flag.Verbose {
		log.SetOutput(os.Stderr)
	}
}

// Print calls same name func from std log package
func Print(v ...interface{}) {
	log.Print(v...)
}

// Printf calls same name func from std log package
func Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

// Println calls same name func from std log package
func Println(v ...interface{}) {
	log.Println(v...)
}

// Fatal calls same name func from std log package
func Fatal(v ...interface{}) {
	log.Fatal(v...)
}

// Fatalf calls same name func from std log package
func Fatalf(format string, v ...interface{}) {
	log.Fatalf(format, v...)
}

// Fatalln calls same name func from std log package
func Fatalln(v ...interface{}) {
	log.Fatalln(v...)
}

// Panic calls same name func from std log package
func Panic(v ...interface{}) {
	log.Panic(v...)
}

// Panicf calls same name func from std log package
func Panicf(format string, v ...interface{}) {
	log.Panicf(format, v...)
}

// Panicln calls same name func from std log package
func Panicln(v ...interface{}) {
	log.Panicln(v...)
}
