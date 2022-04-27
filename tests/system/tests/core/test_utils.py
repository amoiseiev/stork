from datetime import datetime, timedelta
from core.utils import memoize, wait_for_success, NoSuccessException


def test_memoize():
    """Memoized function should be executed only one for a specific arguments"""
    # Arrange
    class Foo:
        def __init__(self, bar):
            self._bar = bar
            self._call_count = 0

        @memoize
        def method(self, baz):
            self._call_count += 1
            return baz + self._bar

    bob = Foo(1)
    alice = Foo(2)

    # Act & Assert
    assert bob.method(3) == 4
    assert bob.method(3) == 4
    assert bob._call_count == 1
    assert bob.method(4) == 5
    assert bob._call_count == 2

    assert alice.method(3) == 5
    assert alice.method(3) == 5
    assert alice._call_count == 1
    assert alice.method(4) == 6
    assert alice.method(4) == 6
    assert bob._call_count == 2


def test_wait_for_instant_success():
    """Waiting for a function that just return value. It should no thrown."""
    # Arrange
    call_count = 0

    @wait_for_success()
    def f():
        nonlocal call_count
        call_count += 1
        return 42

    # Act
    res = f()

    # Assert
    assert res == 42
    assert call_count == 1


def test_wait_for_success_with_retries():
    """A function fails initially, but the decorator repeats execution until
    success happens."""
    # Arrange
    call_count = 0

    @wait_for_success(max_tries=5, sleep_time=0)
    def f():
        nonlocal call_count
        call_count += 1
        if call_count <= 3:
            raise NoSuccessException()
        return 42

    # Act
    res = f()

    # Assert
    assert res == 42
    assert call_count == 4


def test_wait_for_success_use_sleep_time():
    """The decorator waits a specific time between retries."""
    # Arrange
    call_count = 0
    last_call = datetime.min
    delta = timedelta(milliseconds=250)

    @wait_for_success(max_tries=5, sleep_time=delta)
    def f():
        nonlocal call_count, last_call
        now = datetime.now()
        assert now - last_call >= delta
        last_call = now
        call_count += 1

        raise NoSuccessException()

    # Act
    try:
        f()
    except:
        pass

    # Assert
    assert call_count == 5


def test_wait_for_success_with_retries_use_custom_expected_exception():
    """A function fails initially, but the decorator repeats execution until
    success happens. The function throws a custom but expected exception."""
    # Arrange
    call_count = 0

    @wait_for_success(LookupError, max_tries=5, sleep_time=0)
    def f():
        nonlocal call_count
        call_count += 1
        if call_count <= 3:
            raise LookupError()
        return 42

    # Act
    res = f()

    # Assert
    assert res == 42
    assert call_count == 4


def test_wait_for_no_success_use_unexpected_exception():
    """A function fails and throws a custom, unexpected exception. The
    decorator doesn't retry the execution."""
    # Arrange
    call_count = 0

    @wait_for_success(max_tries=5, sleep_time=0)
    def f():
        nonlocal call_count
        call_count += 1
        raise LookupError()

    # Act
    try:
        f()
    except LookupError:
        pass

    # Assert
    assert call_count == 1


def test_wait_for_no_success_with_retries():
    """A function fails initially, and the decorator repeats execution but
    without success. It throws exceptions after exceeding the number of
    retries."""
    # Arrange
    call_count = 0

    @wait_for_success(max_tries=5, sleep_time=0)
    def f():
        nonlocal call_count
        call_count += 1
        raise NoSuccessException()

    # Act
    try:
        f()
    except TimeoutError:
        pass

    # Assert
    assert call_count == 5


def test_wait_for_no_success_with_retries_use_custom_expected_exception():
    """A function fails initially, and the decorator repeats execution but
    without success. It throws exceptions after exceeding the number of
    retries. The function throws a custom but expected exception."""
    # Arrange
    call_count = 0

    @wait_for_success(LookupError, max_tries=5, sleep_time=0)
    def f():
        nonlocal call_count
        call_count += 1
        raise LookupError()

    # Act
    try:
        f()
    except TimeoutError:
        pass

    # Assert
    assert call_count == 5