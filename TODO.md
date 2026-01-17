# TODO: Implement String Handler Methods

- [x] APPEND: Append value to key's string (create if not exists), return new length
- [x] GETRANGE: Return substring from start to end indices (support negative indices)
- [x] SETRANGE: Overwrite string at offset with value (pad with zeros if needed), return new length
- [x] SETNX: Set key only if it doesn't exist, return 1 if set, 0 if exists
- [x] SETEX: Set key with expiration in seconds
- [x] PSETEX: Set key with expiration in milliseconds
- [x] GETSET: Return old value and set new value
- [x] GETEX: Get value and optionally set expiration (EX, PX, EXAT, PXAT, PERSIST options)
- [x] GETDEL: Get value and delete key
- [x] INCRBYFLOAT: Increment key's float value by specified amount
- [x] MSETNX: Set multiple keys only if none exist, return 1 if all set, 0 if any exist

# TODO: Implement List Handler Methods

- [x] LSET: Set element at index
- [x] LINSERT: Insert before/after pivot
- [x] LREM: Remove elements by value
- [x] LTRIM: Trim list to range
- [x] RPOPLPUSH: Pop from one list, push to another
- [x] LMOVE: Move element between lists
- [x] LPOS: Get index of element
- [x] BLPOP: Blocking left pop
- [x] BRPOP: Blocking right pop
- [x] BLMOVE: Blocking list move
- [x] Update handlers.go with new mappings
