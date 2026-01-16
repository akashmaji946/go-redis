from .client import get_client

def GeoAdd(key, *items):
    """Add geospatial items to a sorted set."""
    return get_client().send_command("GEOADD", key, *items)

def GeoPos(key, *members):
    """Return the positions (longitude, latitude) of all specified members in a geospatial index."""
    return get_client().send_command("GEOPOS", key, *members)

def GeoDist(key, member1, member2, unit=None):
    """Return the distance between two members in a geospatial index."""
    args = ["GEODIST", key, member1, member2]
    if unit:
        args.append(unit)
    return get_client().send_command(*args)

def GeoHash(key, *members):
    """Return the geohash strings for the specified members in a geospatial index."""
    return get_client().send_command("GEOHASH", key, *members)

def GeoRadius(key, longitude, latitude, radius, unit, *options):
    """Query members within a radius of a given point in a geospatial index."""
    return get_client().send_command("GEORADIUS", key, longitude, latitude, radius, unit, *options)

def GeoSearch(key, *search_args):
    """Search for members within a radius or rectangular box in a geospatial index."""
    return get_client().send_command("GEOSEARCH", key, *search_args)

def GeoSearchStore(destination, source, *search_args):
    """Search for members within a radius or rectangular box and store the results in a destination key."""
    return get_client().send_command("GEOSEARCHSTORE", destination, source, *search_args)
