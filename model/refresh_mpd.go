package model

import (
	"errors"
	"log"
	"path"
	"strings"
	"time"

	"github.com/werkshy/gompd/mpd"
	"github.com/werkshy/pickup/config"
)

func RefreshMpd(conf *config.Config) (Collection, error) {
	conn, err := mpd.DialAuthenticated("tcp", *conf.MpdAddress, *conf.MpdPassword)
	if err != nil {
		return Collection{}, err
	}

	log.Println("Getting mpd files")
	t0 := time.Now()
	files, err := conn.GetFiles()
	if err != nil {
		return Collection{}, err
	}

	log.Printf("Getting %v files from mpd took %d ms", len(files),
		time.Since(t0)/time.Millisecond)
	t1 := time.Now()

	// Files come back from mpd sorted, so we can track the current
	// artists/albums as we iterate through the files.
	rootCategory := NewCategory("Music")
	collection := Collection{
		make([]*Category, 0),
	}
	collection.addCategory(rootCategory)

	var currentArtist *Artist
	var currentAlbum *Album
	var currentCategory *Category = rootCategory
	for _, file := range files {
		category, artist, album, track, err := PathToParts(file)
		if err != nil {
			log.Printf("Error at %s: %v\n", file, err)
			continue
		}

		// Occassionally I have e.g. _mp3/ folders that I want to ignore
		if strings.HasPrefix(album, "_") {
			log.Printf("Ignoring album '%s'\n", file)
			continue
		}

		// handle currentAlbum
		if currentAlbum == nil {
			currentAlbum = NewAlbum(album)
			currentAlbum.Path = path.Dir(file)
		} else if currentAlbum.Name != album {
			// handle finished album
			wrapUpAlbum(currentAlbum, currentArtist, currentCategory)
			currentAlbum = NewAlbum(album)
			currentAlbum.Path = path.Dir(file)
		}

		// Handle currentArtist
		if artist != "" {
			// Create a new artist if the artist has changed
			if currentArtist == nil {
				currentArtist = NewArtist(artist)
			} else if currentArtist.Name != artist {
				// handle finished artist
				wrapUpArtist(currentArtist, currentCategory)
				currentArtist = NewArtist(artist)
			}
		} else {
			// Looking at a bare album
			if currentArtist != nil {
				// handle finished album if currentArtist != nil
				wrapUpArtist(currentArtist, currentCategory)
			}
			currentArtist = nil
		}

		// handle currentCategory
		if category != "" {
			// This file is part of a subcategory
			if currentCategory == rootCategory {
				currentCategory = NewCategory(category)
			} else if currentCategory.Name != category {
				// handle finished subcategory
				wrapUpCategory(currentCategory, &collection)
				currentCategory = NewCategory(category)
			}
		} else {
			// this file is not in a subcategory, revert to root category
			if currentCategory != rootCategory {
				// handle finished sub category
				wrapUpCategory(currentCategory, &collection)
			}
			currentCategory = rootCategory
		}
		currentTrack := Track{track, file, currentAlbum.Name, ""}
		if currentAlbum != nil {
			currentTrack.Album = currentAlbum.Name
		}
		currentAlbum.Tracks = append(currentAlbum.Tracks, &currentTrack)
	}

	// handle final category, artist and album
	if currentAlbum != nil {
		wrapUpAlbum(currentAlbum, currentArtist, currentCategory)
	}
	if currentArtist != nil {
		// handle finished artist
		wrapUpArtist(currentArtist, currentCategory)
	}
	if currentCategory != nil {
		// handle finished category
		wrapUpCategory(currentCategory, &collection)
	}

	log.Printf("Sorting mpd results took %d ms\n", time.Since(t1)/time.Millisecond)
	log.Printf("Found %d categories\n", len(collection.Categories))
	//for _, category := range collection.Categories {
	//	log.Printf("    %s\n", category.Name)
	//}
	return collection, err
}

func PathToParts(path string) (category string, artist string, album string, track string, err error) {
	parts := strings.Split(path, "/")
	nparts := len(parts)
	if nparts < 2 {
		log.Printf("Can't handle '%s'\n", path)
		return "", "", "", "", errors.New("Too few parts")
	}
	track = parts[nparts-1]
	album = parts[nparts-2]

	// Occassionally I have e.g. _mp3/ folders that I want to ignore
	if strings.HasPrefix(album, "_") {
		//log.Printf("Ignoring album '%s'\n", file)
		return "", "", album, track, nil
	}

	npartsWithArtist := 3 // expect artist, album, track
	// If the path begins with _, it's a subcategory e.g. _Soundtracks
	if strings.HasPrefix(path, "_") {
		npartsWithArtist = 4 // category, artist, album, track
	}
	// Sanity check the path for too many or too few parts
	// one less that nparts is OK, it means a bare album
	if len(parts) < npartsWithArtist-1 || len(parts) > npartsWithArtist {
		log.Printf("%s has %d parts", path, len(parts))
		return "", "", "", "", errors.New("Wrong number of parts")
	}

	// Handle currentArtist
	if nparts == npartsWithArtist {
		artist = parts[nparts-3]
	}

	// handle currentCategory
	if strings.HasPrefix(path, "_") {
		// This file is part of a subcategory
		category = parts[0]
	}
	return
}

func wrapUpAlbum(album *Album, artist *Artist, category *Category) {
	album.Category = category.Name
	if artist != nil {
		album.Artist = artist.Name
		artist.Albums = append(artist.Albums, album)
	} else { // bare album, no artist
		category.Albums = append(category.Albums, album)
	}
}

func wrapUpArtist(artist *Artist, category *Category) {
	category.Artists = append(category.Artists, artist)
}

func wrapUpCategory(category *Category, collection *Collection) {
	log.Printf("Wrapping up category: %s", category.Name)
	collection.addCategory(category)
}
