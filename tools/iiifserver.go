package tools

import (
	"errors"
	"flag"
	"fmt"
	"github.com/facebookgo/grace/gracehttp"
	iiifcache "github.com/go-iiif/go-iiif/cache"
	iiifconfig "github.com/go-iiif/go-iiif/config"
	iiifdriver "github.com/go-iiif/go-iiif/driver"
	iiifhttp "github.com/go-iiif/go-iiif/http"
	iiiflevel "github.com/go-iiif/go-iiif/level"
	iiifsource "github.com/go-iiif/go-iiif/source"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type IIIFServerTool struct {
	Tool
}

func NewIIIFServerTool() (Tool, error) {

	t := &IIIFServerTool{}
	return t, nil
}

func (t *IIIFServerTool) Run() error {

	var cfg = flag.String("config", "", "Path to a valid go-iiif config file")
	var host = flag.String("host", "localhost", "Bind the server to this host")
	var port = flag.Int("port", 8080, "Bind the server to this port")
	var example = flag.Bool("example", false, "Add an /example endpoint to the server for testing and demonstration purposes")
	var root = flag.String("example-root", "example", "An explicit path to a folder containing example assets")

	flag.Parse()

	if *cfg == "" {
		return errors.New("Missing config file")
	}

	config, err := iiifconfig.NewConfigFromFlag(*cfg)

	if err != nil {
		return err
	}

	driver, err := iiifdriver.NewDriverFromConfig(config)

	if err != nil {
		return err
	}

	/*

		See this - we're just going to make sure we have a valid source
		before we start serving images (20160901/thisisaaronland)

	*/

	_, err = iiifsource.NewSourceFromConfig(config)

	if err != nil {
		return err
	}

	_, err = iiiflevel.NewLevelFromConfig(config, *host)

	if err != nil {
		return err
	}

	/*

		Okay now we're going to set up global cache thingies for source images
		and derivatives mostly to account for the fact that in-memory cache
		thingies need to be... well, global

	*/

	images_cache, err := iiifcache.NewImagesCacheFromConfig(config)

	if err != nil {
		return err
	}

	derivatives_cache, err := iiifcache.NewDerivativesCacheFromConfig(config)

	if err != nil {
		return err
	}

	info_handler, err := iiifhttp.InfoHandler(config, driver)

	if err != nil {
		return err
	}

	image_handler, err := iiifhttp.ImageHandler(config, driver, images_cache, derivatives_cache)

	if err != nil {
		return err
	}

	ping_handler, err := iiifhttp.PingHandler()

	if err != nil {
		return err
	}

	expvar_handler, err := iiifhttp.ExpvarHandler(*host)

	if err != nil {
		return err
	}

	router := mux.NewRouter()

	router.HandleFunc("/ping", ping_handler)
	router.HandleFunc("/debug/vars", expvar_handler)

	// https://github.com/go-iiif/go-iiif/issues/4

	router.HandleFunc("/{identifier:.+}/info.json", info_handler)
	router.HandleFunc("/{identifier:.+}/{region}/{size}/{rotation}/{quality}.{format}", image_handler)

	if *example {

		abs_path, err := filepath.Abs(*root)

		if err != nil {
			return err
		}

		_, err = os.Stat(abs_path)

		if os.IsNotExist(err) {
			return err
		}

		example_handler, err := iiifhttp.ExampleHandler(abs_path)

		if err != nil {
			return err
		}

		router.HandleFunc("/example/{ignore:.*}", example_handler)
	}

	endpoint := fmt.Sprintf("%s:%d", *host, *port)

	log.Printf("Listening for requests at %s\n", endpoint)

	err = gracehttp.Serve(&http.Server{Addr: endpoint, Handler: router})

	if err != nil {
		return err
	}

	return nil
}
