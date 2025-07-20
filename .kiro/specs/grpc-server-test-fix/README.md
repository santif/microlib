# gRPC Server Test Fix

## Issue Description

The gRPC server implementation in the MicroLib framework was experiencing a test timeout issue in the `TestStartAndShutdown` test. The test would time out after 10 minutes, indicating a deadlock in the server's shutdown process.

## Root Cause

The root cause of the issue was a deadlock in the `Shutdown()` method of the gRPC server implementation. The method was acquiring a write lock on the `startedMu` mutex and then calling the `Address()` method, which tries to acquire a read lock on the same mutex. This created a classic deadlock scenario.

Stack trace analysis showed:
1. The `Shutdown()` method acquired a write lock on `s.startedMu`
2. While holding this lock, it called `s.Address()` when logging the server shutdown
3. The `Address()` method tried to acquire a read lock on the same mutex `s.startedMu`
4. Since the write lock is exclusive, the read lock could never be acquired, resulting in a deadlock

## Fix Implementation

The fix involved several changes:

1. **Fixed the deadlock in the Shutdown method**:
   - Modified the `Shutdown()` method to get the server address before acquiring the lock
   - This prevents the deadlock by ensuring we don't call `Address()` while holding the write lock

2. **Improved resource cleanup during shutdown**:
   - Added explicit closing of the listener
   - Added proper nulling of references to release resources
   - Enhanced logging during the shutdown process

3. **Added better timeout handling for forced shutdown**:
   - Improved the handling of shutdown timeouts
   - Added appropriate logging for timeout events
   - Ensured all resources are released even in forced shutdown scenarios

4. **Enhanced tests**:
   - Updated the `TestStartAndShutdown` test to use a shorter timeout
   - Added a new test for forced shutdown due to timeout
   - Added a test for concurrent shutdown calls to ensure thread safety

## Verification

The fix was verified by running the tests and ensuring they complete successfully within a reasonable time frame. The tests now properly test both graceful shutdown and forced shutdown scenarios.

## Lessons Learned

1. Be careful when calling methods that acquire locks while already holding a lock
2. Document locking behavior in method comments to prevent similar issues
3. Use timeouts in tests to catch potential deadlocks early
4. Ensure proper resource cleanup during shutdown, especially in error cases