import goredis as grs

# Initialize connection
grs.Connect(host='127.0.0.1', port=7379)

# Authenticate if required
grs.Auth("root", "dsl")

# Select DB and perform operations
grs.Select(0)
grs.Set("my_key", "Hello from Python!")
print(grs.Get('my_key'))
print(grs.Del('my_key'))
print(grs.Get('my_key'))
# List operations
grs.LPush("my_list", "item1", "item2")
print(f"List: {grs.LGet('my_list')}")

# Transaction example
grs.Watch("my_key")
grs.Multi()
grs.Set("my_key", "new_value")
results = grs.Exec()
print(f"Transaction Results: {results}")
