# Documentation Update TODO

## README.md Updates
- [x] Add HyperLogLog category with PFADD, PFCOUNT, PFDEBUG, PFMERGE commands
- [x] Add missing ECHO command to Connection section
- [x] Add missing HMGET command to Hash section
- [x] Add missing FLUSHALL command to Server section
- [x] Add missing USERDEL command to Connection section

## DOCS.md Updates
- [x] Add HyperLogLog Operations section with detailed command table
- [x] Add ECHO command to Server & Connection section
- [x] Add HMGET command to Hash Operations section
- [x] Add FLUSHALL command to Server & Connection section
- [x] Add USERDEL command to User Management section
- [x] Update command descriptions to match commands.json

## Verification
- [x] Verify all commands from commands.json are documented
- [x] Verify consistency between README.md and DOCS.md

## Summary of Changes

### README.md
- Added HyperLogLog section with 4 commands: PFADD, PFCOUNT, PFDEBUG, PFMERGE
- Added ECHO to Connection section
- Added USERDEL to Connection section
- Added HMGET to Hash section
- Added FLUSHALL to Server section
- Reorganized commands alphabetically within sections
- Renamed "ZSet" to "Sorted Set (ZSet)" for clarity

### DOCS.md
- Added new HyperLogLog Operations section with detailed command table
- Added ECHO command to Server & Connection section
- Added HMGET command to Hash Operations section
- Added USERDEL command to User Management section
- Added FLUSHALL and DROPDB (as separate entries) to Monitoring & Information section
- Updated all command descriptions to be more comprehensive based on commands.json
- Separated alias commands into their own rows for clarity (e.g., PUB, SUB, UNSUB, etc.)
- Added information about admin privileges requirements where applicable
