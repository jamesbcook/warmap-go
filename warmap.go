//Rewrite of Tom Steele's node.JS version

package main

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"

	"io/ioutil"

	"golang.org/x/net/html/charset"
)

//To-Do
//Add Signal bleed based signal strength

//Type Definitions and variables

//tpl defines the HTML template
const tpl = `
<html>
  <head>
    <title>WarMap</title>
    <script type="text/javascript" src="http://ajax.googleapis.com/ajax/libs/jquery/1.6.4/jquery.min.js"></script>
    <script type="text/javascript" src="http://maps.google.com/maps/api/js?sensor=true"></script>
    <script type="text/javascript" src="http://hpneo.github.io/gmaps/gmaps.js"></script>
    <link href="http://netdna.bootstrapcdn.com/bootstrap/3.0.0/css/bootstrap.min.css" rel="stylesheet">
    <style>
      #map {
        display: block;
        width: 100%;
        height: 700;
      }
    </style>
    <script>
      $(document).ready(function() {
        var isEditable = false;
        $('#editButton').click(function() {
          if (isEditable === false) {
            isEditable = true;
          } else {
            isEditable = false;
          }
          drawMap();
        });
        function drawMap() {
          var map = new GMaps({
            el: '#map',
            lat: {{.Lat}},
            lng: {{.Lng}},
            zoom: 16
          });
          paths = {{.Paths}};
          map.drawPolygon({
            paths: paths,
            fillColor: '#3366FF',
            fillOpacity: 0.5,
            strokeColor: '#3366ff',
            strokeOpacity: 1.0,
            strokeWeight: 1,
            strokePosition: 'OUTSIDE',
            editable: isEditable
          });
        }
        drawMap();
      });
    </script>
  </head>
  <body>
    <div class="container" style="padding-top: 80px">
      <div class="row">
        <div class="col-xs-12">
          <p>Displaying {{.PathLength}} points.</p>
        </div>
      </div>
      <div class="row">
        <div class="col-xs-11">
          <div id="map"></div>
        </div>
        <div class="col-xs-1">
          <button type="button" id="editButton" class="btn btn-primary" data-toggle="button">Editable</button>
        </div>
      </div>
    </div>
  </body>
</html>
`

////////////////////
//Type Definitions//
///////////////////

// Points defines a []Point array
type Points []Point

//Point holds X, Y coordinates
type Point struct {
	X, Y float64
}

//Page Holds the Values for html template
type Page struct {
	Lat        float64
	Lng        float64
	Paths      [][]float64
	PathLength int
}

//GPSRun defines a struct to hold the top level of the
//xml tree
type GPSRun struct {
	XMLName    xml.Name   `xml:"gps-run"`
	GPSVersion string     `xml:"gps-version,attr"`
	StartTime  string     `xml:"start-time,attr"`
	GPSPoints  []GPSPoint `xml:"gps-point"`
}

//GPSPoint defines a struct to hold the values
//of gps-point
type GPSPoint struct {
	XMLName   xml.Name `xml:"gps-point"`
	Bssid     string   `xml:"bssid,attr"`
	Lat       float64  `xml:"lat,attr"`
	Lon       float64  `xml:"lon,attr"`
	Source    string   `xml:"source,attr"`
	TimeSec   int      `xml:"time-sec,attr"`
	TimeUSec  int      `xml:"time-usec,attr"`
	Spd       float64  `xml:"spd,attr"`
	Heading   float64  `xml:"heading,attr"`
	Fix       int      `xml:"fix,attr"`
	Alt       float64  `xml:"alt,attr"`
	SignalDbm int      `xml:"signal_dbm,attr"`
	NoiseDbm  int      `xml:"noise_dbm,attr"`
}

/////////////
//Functions//
////////////

//checkError is a generic error check function
func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

//parseXML parses the specified XML file and returns a GPSRun struct
//containing its values
func parseXML(file string) (target GPSRun) {
	xmlFile, err := os.Open(file)
	if err != nil {
		panic("Ensure the GPSXML file exists")
	}
	defer xmlFile.Close()
	var xmlData io.Reader = bufio.NewReader(xmlFile)
	decoder := xml.NewDecoder(xmlData)
	decoder.CharsetReader = charset.NewReaderLabel
	err = decoder.Decode(&target)
	checkError(err)
	return
}

//parseCoords parses a GPSPoint array and puts them into a Point
//struct that is then placed into a Points struct
func parseCoords(gpspoints []GPSPoint) (points Points) {
	for i := 0; i < len(gpspoints); i++ {
		points = append(points, Point{gpspoints[i].Lon, gpspoints[i].Lat})
	}
	return
}

//filterBSSID returns all GPSPoint structs that have a particular bssid field
func filterBSSID(gpsPoints []GPSPoint, bssid []string) (filteredPoints []GPSPoint) {
	for i := 0; i < len(gpsPoints); i++ {
		for n := 0; n < len(bssid); n++ {
			if gpsPoints[i].Bssid == bssid[n] {
				filteredPoints = append(filteredPoints, gpsPoints[i])
			}
		}
	}
	return
}

//Swap swaps the location of two Point structs in a Points struct
func (points Points) Swap(i, j int) {
	points[i], points[j] = points[j], points[i]
}

//Len is custom length definition for Points
func (points Points) Len() int {
	return len(points)
}

//Less sorts Points by x and, if equal, by y
func (points Points) Less(i, j int) bool {
	if points[i].X == points[j].X {
		return points[i].Y < points[j].Y
	}
	return points[i].X < points[j].X
}

//crossProduct returns the modulo (and sign) of the cross product between vetors OA and OB
func crossProduct(O, A, B Point) float64 {
	return (A.X-O.X)*(B.Y-O.Y) - (A.Y-O.Y)*(B.X-O.X)
}

// findConvexHull returns a slice of Point with a convex hull
// it is counterclockwise and starts and ends at the same point
func findConvexHull(points Points) Points {
	n := len(points)  // number of points to find convex hull
	var result Points // final result
	count := 0        // size of our convex hull (number of points added)

	// lets sort our points by x and if equal by y
	sort.Sort(points)

	if n == 0 {
		return result
	}

	// add the first element:
	result = append(result, points[0])
	count++

	// find the lower hull
	for i := 1; i < n; i++ {
		// remove points which are not part of the lower hull
		for count > 1 && crossProduct(result[count-2], result[count-1], points[i]) < 0.00000000000000001 {
			count--
			result = result[:count]
		}

		// add a new better point than the removed ones
		result = append(result, points[i])
		count++
	}

	count0 := count // our base counter for the upper hull

	// find the upper hull
	for i := n - 2; i >= 0; i-- {
		// remove points which are not part of the upper hull
		for count-count0 > 0 && crossProduct(result[count-2], result[count-1], points[i]) < 0.00000000000000001 {
			count--
			result = result[:count]
		}

		// add a new better point than the removed ones
		result = append(result, points[i])
		count++
	}

	return result
}

//parseBssid takes a filename or comma seperated list of BSSIDs
//and outputs an array containing the parsed BSSIDs
func parseBssid(bssids string) (tempBssidSlice []string) {
	var bssidSlice []string
	r, err := regexp.Compile("(([A-Z0-9]{2}:)){5}[A-Z0-9]{2}")
	checkError(err)
	if _, err := os.Stat(bssids); os.IsNotExist(err) {
		bssidSlice = strings.Split(bssids, ",")
	} else {
		bssidFileData, err := ioutil.ReadFile(bssids)
		checkError(err)
		bssidSlice = strings.Split(string(bssidFileData), "\n")
	}
	for i := 0; i < len(bssidSlice); i++ {
		if r.MatchString(bssidSlice[i]) {
			tempBssidSlice = append(tempBssidSlice, bssidSlice[i])
		}
	}
	if len(tempBssidSlice) == 0 {
		log.Fatal("Looks like you didn't have any correctly formatted SSIDs")
	}
	return
}

//formatToTemplate formats a Points struct into a bi-dimensional float64
//array which makes it easier to put into template
func formatToTemplate(points Points) (pathsData [][]float64) {
	pathsData = make([][]float64, len(points))
	for i := 0; i < len(points); i++ {
		pathsData[i] = []float64{points[i].Y, points[i].X}
	}
	return pathsData
}

//populateTemplate populates the html template
func populateTemplate(points [][]float64) []byte {
	var page Page
	var tplBuffer bytes.Buffer
	page.Lat = points[0][0]
	page.Lng = points[0][1]
	page.PathLength = len(points)
	page.Paths = points
	t, err := template.New("webpage").Parse(tpl)
	checkError(err)
	err = t.Execute(&tplBuffer, page)
	checkError(err)
	return tplBuffer.Bytes()
}

func main() {
	//Parse command line arguments
	var gpsxmlFile = flag.String("f", "", "Gpsxml input file")
	var bssid = flag.String("b", "", "File or comma seperated list of bssids")
	var outFile = flag.String("o", "", "Html Output file")
	flag.Parse()
	if !flag.Parsed() || !(flag.NArg() == 3) {
		fmt.Println("Usage: warmap -f <Kismet gpsxml file> -b <File or List of BSSIDs> -o <HTML output file>")
		os.Exit(1)
	}
	xmlTree := parseXML(*gpsxmlFile)
	bssidData := parseBssid(*bssid)
	filteredPoints := filterBSSID(xmlTree.GPSPoints, bssidData)
	parsedCoords := parseCoords(filteredPoints)
	templateBuffer := populateTemplate(formatToTemplate(findConvexHull(parsedCoords)))
	ioutil.WriteFile(*outFile, templateBuffer, 0644)
}
