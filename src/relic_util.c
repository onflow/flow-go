/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007, 2008, 2009 RELIC Authors
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file
 * for contact information.
 *
 * RELIC is free software; you can redistribute it and/or
 * modify it under the terms of the GNU Lesser General Public
 * License as published by the Free Software Foundation; either
 * version 2.1 of the License, or (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Implementation of useful configuration routines.
 *
 * @version $Id$
 * @ingroup relic
 */

#include <stdio.h>
#include <stdarg.h>

#include "relic_conf.h"
#include "relic_util.h"
#include "relic_types.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Table to convert between digits and chars.
 */
static const char conv_table[] =
		"0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz+/";

/**
 * Buffer to hold printed messages.
 */
static char buffer[64 + 1];

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

uint32_t util_conv_big(uint32_t i) {
#ifdef BIGED
	return i;
#else
	uint32_t i1, i2, i3, i4;
	i1 = i & 0xFF;
	i2 = (i >> 8) & 0xFF;
	i3 = (i >> 16) & 0xFF;
	i4 = (i >> 24) & 0xFF;

	return ((uint32_t)i1 << 24) | ((uint32_t)i2 << 16) | ((uint32_t)i3 << 8) | i4;
#endif
}

char util_conv_char(dig_t i) {
	return conv_table[i];
}

int util_bits_dig(dig_t a) {
#if WORD == 8 || WORD == 16
	static const unsigned char table[16] = {
		0, 1, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4
	};
#endif
#if WORD == 8
	if (a >> 4 == 0) {
		return table[a & 0xF];
	} else {
		return table[a >> 4] + 4;
	}
	return 0;
#elif WORD == 16
	int offset;

	if (a >= ((dig_t)1 << 8)) {
		offset = 8;
	} else {
		offset = 0;
	}
	a = a >> offset;
	if (a >> 4 == 0) {
		return table[a & 0xF] + offset;
	} else {
		return table[a >> 4] + 4 + offset;
	}
	return 0;
#elif WORD == 32
	return DIGIT - __builtin_clz(a);
#elif WORD == 64
	return DIGIT - __builtin_clzl(a);
#endif
}

void util_printf(char *format, ...) {
#ifndef QUIET
#if ARCH == AVR
	char *pointer = &buffer[1];
	va_list list;
	va_start(list, format);
	vsnprintf(pointer, 128, format, list);
	buffer[0] = (unsigned char)2;
	va_end(list);
#else
	va_list list;
	va_start(list, format);
	vsnprintf(buffer, 64, format, list);
	printf("%s", buffer);
	va_end(list);
#endif
#endif
}
