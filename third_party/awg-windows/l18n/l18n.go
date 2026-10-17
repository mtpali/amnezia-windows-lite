/* SPDX-License-Identifier: MIT
 * Copyright (C) 2019-2022 WireGuard LLC. All Rights Reserved.
 */
package l18n

import "fmt"

func Sprintf(format string, a ...any) string { return fmt.Sprintf(format, a...) }
func EnumerationSeparator() string           { return ", " }
func UnitSeparator() string                  { return ", " }
