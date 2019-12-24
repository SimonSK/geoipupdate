# GeoIP Update
This is a fork of the official MaxMind GeoIP Update library (maxmind/geoipupdate).

Following changes are made to suit my needs:
* Reverted maxmind/geoipupdate#60 to enable downloading GeoLite2 databases without a valid credential
* When a new database is downloaded, the filename now includes a unique identifier to allow keeping old versions.
  * Last modified timestamp of the downloaded database is used as the unique identifier and appended to the original filename.
  * e.g. `GeoLite2-City.mmdb-20191217T200549Z` 
* A symlink to the newly downloaded file is created.
  * e.g. `GeoLite2-City.mmdb@ -> GeoLite2-City.mmdb-20191217T200549Z`
