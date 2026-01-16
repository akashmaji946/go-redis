package goredis

// GeoAdd adds geospatial items to a sorted set.
// It returns the server's response or an error if the command fails.
func GeoAdd(key string, items ...interface{}) (interface{}, error) {
	cmdArgs := append([]interface{}{"GEOADD", key}, items...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// GeoPos returns the positions (longitude, latitude) of all specified members in a geospatial index.
// It returns the server's response or an error if the command fails.
func GeoPos(key string, members ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"GEOPOS", key}, toInterfaceSlice(members)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// GeoDist returns the distance between two members in a geospatial index.
// It returns the server's response or an error if the command fails.
func GeoDist(key, member1, member2 string, unit ...string) (interface{}, error) {
	cmdArgs := []interface{}{"GEODIST", key, member1, member2}
	if len(unit) > 0 {
		cmdArgs = append(cmdArgs, unit[0])
	}
	return mustGetClient().SendCommand(cmdArgs...)
}

// GeoHash returns the geohash strings for the specified members in a geospatial index.
// It returns the server's response or an error if the command fails.
func GeoHash(key string, members ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"GEOHASH", key}, toInterfaceSlice(members)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// GeoRadius queries members within a radius of a given point in a geospatial index.
// It returns the server's response or an error if the command fails.
func GeoRadius(key string, longitude, latitude, radius float64, unit string, options ...interface{}) (interface{}, error) {
	cmdArgs := []interface{}{"GEORADIUS", key, longitude, latitude, radius, unit}
	cmdArgs = append(cmdArgs, options...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// GeoSearch searches for members within a radius or rectangular box in a geospatial index.
// It returns the server's response or an error if the command fails.
func GeoSearch(key string, searchArgs ...interface{}) (interface{}, error) {
	cmdArgs := append([]interface{}{"GEOSEARCH", key}, searchArgs...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// GeoSearchStore searches for members within a radius or rectangular box and stores the results in a destination key.
// It returns the server's response or an error if the command fails.
func GeoSearchStore(destination, source string, searchArgs ...interface{}) (interface{}, error) {
	cmdArgs := append([]interface{}{"GEOSEARCHSTORE", destination, source}, searchArgs...)
	return mustGetClient().SendCommand(cmdArgs...)
}
