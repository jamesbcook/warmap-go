# warmap-go

Warmap takes a Kismet gpsxml file and a set of BSSIDs and creates a polygon of coordinates using the convex hull algorithm. This polygon is overlayed over a Google Maps generated map to show the coverage area of the specified BSSID.

##Usage:##

go run warmap -f [Kismet gpsxml file] -b [File or Comma-seperated List of BSSIDs] -o [HTML output file]

Binaries for all platforms can be found <a href="https://github.com/rmikehodges/warmap-go/releases">here</a>
