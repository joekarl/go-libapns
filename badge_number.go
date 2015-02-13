package apns

import (
	"errors"
	"strconv"
)

type BadgeNumber struct {
	number int
	set    bool
}

func (b *BadgeNumber) Number() int {
	return b.number
}

func (b *BadgeNumber) IsSet() bool {
	return b.set
}

func (b *BadgeNumber) UnSet() {
	b.number = 0
	b.set = false
}

func (b *BadgeNumber) Set(number int) error {
	if number < 0 {
		return errors.New("Number must be >= 0")
	}

	b.number = number
	b.set = true
	return nil
}

func (b BadgeNumber) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Itoa(b.number)), nil
}

func (b *BadgeNumber) UnmarshalJSON(data []byte) error {
	val, err := strconv.ParseInt(string(data), 10, 32)
	if err != nil {
		return errors.New("Error unmarshalling BadgeNumber, cannot convert []byte to int32")
	}

	// Since the point of this type is to
	// allow proper inclusion of 0 for int
	// types while respecting omitempty,
	// assume that set==true if there is
	// a value to unmarshal
	*b = BadgeNumber{
		number: int(val),
		set:    true,
	}
	return nil
}

func NewBadgeNumber(number int) BadgeNumber {
	return BadgeNumber{
		number: number,
		set:    true,
	}
}
