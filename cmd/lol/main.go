// Command lol runs Lands of Lore: The Throne of Chaos (MAIN.EXE of the DOS CD version 1.02) from
// the original data files, ported to Go.
//
//	lol [-data DIR]            play with the game files in DIR (default $GOPLAY_DATA, else ./game)
//	lol -music mt32|cm32l|adlib  the music card: the MT-32 the game was written for (the
//	                           default; ROMs from -roms, $GOPLAY_MT32 or ./roms), a CM-32L
//	                           or new MT-32, or the AdLib
//	lol -import SRC -data DIR  copy the game files out of SRC (folder, zip, ISO/BIN/CUE, GOG
//	                           installer, loose files) into DIR and exit
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/memmaker/lands-of-lore-1-go/internal/data"
	"github.com/memmaker/lands-of-lore-1-go/internal/game"
	"github.com/memmaker/lands-of-lore-1-go/internal/host"
	"github.com/memmaker/lands-of-lore-1-go/internal/sound"
	"github.com/retro-ware/gameimport"
)

// markerFiles prove a folder holds Lands of Lore.
var markerFiles = []string{"GENERAL.PAK", "STARTUP.PAK"}

// Permissions of the directories and files written.
const (
	dirMode  = 0o755
	fileMode = 0o644
)

// gameFile says whether name is one of the game's data files (the installer's and CD's
// programs, drivers, documents and pictures are not).
func gameFile(name string) bool {
	switch strings.ToUpper(filepath.Ext(name)) {
	case ".PAK", ".TLK", ".ADL", ".FDT", ".FNT", ".TAB", ".CFG", ".RST", ".TDS", ".LM":
		return true
	}
	return false
}

func main() {
	defaultDir := os.Getenv("GOPLAY_DATA")
	if defaultDir == "" {
		defaultDir = "game"
		if _, err := os.Stat("data"); err == nil {
			defaultDir = "data" // goplay: started in games/lol with no arguments
		}
	}
	dir := flag.String("data", defaultDir, "directory with the original game files")
	source := flag.String("import", "", "import the game files from this folder/zip/ISO/installer into -data and exit")
	scale := flag.Int("scale", 3, "window scale")
	hero := flag.Int("hero", -1, "start a new game with hero 0-3, as MAIN.EXE's argument (Ak'shel, Michael, Kieran, Conrad)")
	music := flag.String("music", musicMT32, "music card: mt32 (old MT-32, as the game was written for), cm32l (CM-32L or new MT-32) or adlib")
	roms := flag.String("roms", os.Getenv("GOPLAY_MT32"), "the MT-32 ROM directory (default: roms/ in the working directory or next to the binary)")
	flag.Parse()

	if *source != "" {
		if err := importGame(*source, *dir); err != nil {
			fmt.Fprintln(os.Stderr, "import:", err)
			os.Exit(1)
		}
		return
	}
	if _, err := os.Stat(filepath.Join(*dir, markerFiles[0])); err != nil {
		fmt.Fprintf(os.Stderr, "no game files in %s; use -import SRC or -data DIR\n", *dir)
		os.Exit(1)
	}
	var gameErr error
	err := host.Run("Lands of Lore", *scale, func() { gameErr = run(*dir, *hero, *music, romDir(*roms)) })
	if err == nil {
		err = gameErr
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// importGame copies the game files found in source into dir.
func importGame(source, dir string) error {
	files, err := gameimport.Find(source, gameFile)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for name := range files {
		have[strings.ToUpper(filepath.Base(name))] = true
	}
	for _, marker := range markerFiles {
		if !have[marker] {
			return fmt.Errorf("no Lands of Lore files (%s) found in %s", marker, source)
		}
	}
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return err
	}
	for name, contents := range files {
		// The CD keeps the files shared by all languages in DATA and each language's own
		// versions, some under the same names (STARTUP.PAK is the intro's there), in
		// DATA/ENG, FRE and GER; the language folders stay apart.
		target := dir
		if lang := strings.ToUpper(filepath.Base(filepath.Dir(name))); data.IsLanguageFolder(lang) {
			target = filepath.Join(dir, lang)
		}
		if err := os.MkdirAll(target, dirMode); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(target, strings.ToUpper(filepath.Base(name))), contents, fileMode); err != nil {
			return err
		}
	}
	return writeCover(dir)
}

// The -music choices.
const (
	musicMT32  = "mt32"
	musicCM32L = "cm32l"
	musicAdLib = "adlib"
)

// run is the game.
func run(dir string, hero int, music, roms string) error {
	g, err := game.New(dir)
	if err != nil {
		return err
	}
	switch music {
	case musicMT32, musicCM32L:
		// The game's patches (LOLSYSEX) were made on the first MT-32; the CM-32L's and the
		// later MT-32's changed sound engine can distort some, so each is asked for by name.
		gen := sound.MT32Old
		if music == musicCM32L {
			gen = sound.MT32New
		}
		mt, err := sound.OpenMT32(roms, gen)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v; playing the AdLib music instead\n", err)
			break
		}
		g.UseMT32(mt)
	case musicAdLib:
	default:
		return fmt.Errorf("unknown -music %q (mt32, cm32l or adlib)", music)
	}
	if hero >= 0 {
		g.Hero = hero
		g.SkipCharGen = true
	}
	g.StartAudio()
	return g.Run()
}

// romDir is -roms, else the first roms/ directory in the working directory or next to the
// binary ("" when there is none).
func romDir(flagDir string) string {
	if flagDir != "" {
		return flagDir
	}
	dirs := []string{"roms"}
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(exe), "roms"))
	}
	for _, d := range dirs {
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			return d
		}
	}
	return ""
}
