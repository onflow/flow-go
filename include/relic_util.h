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
 * @defgroup util Misc utilities.
 */

/**
 * @file
 *
 * Interface of misc utilitles.
 *
 * @version $Id$
 * @ingroup util
 */

#ifndef RELIC_UTIL_H
#define RELIC_UTIL_H

#include "relic_types.h"

/*============================================================================*/
/* Macro definitions                                                          */
/*============================================================================*/

/**
 * Returns the minimum between two numbers.
 *
 * @param[in] A		- the first number.
 * @param[in] B		- the second number.
 * @returns			- the smaller number.
 */
#define MIN(A, B)			((A) < (B) ? (A) : (B))

/**
 * Returns the maximum between two numbers.
 *
 * @param[in] A		- the first number.
 * @param[in] B		- the second number.
 * @returns			- the bigger number.
 */
#define MAX(A, B)			((A) > (B) ? (A) : (B))

/**
 * Splits a bit count in a digit count and an updated bit count.
 *
 * @param[out] B		- the resulting bit count.
 * @param[out] D		- the resulting digit count.
 * @param[out] V		- the bit count.
 * @param[in] L			- the logarithm of the digit size.
 */
#define SPLIT(B, D, V, L)	D = (V) >> (L); B = (V) - ((D) << (L));

/**
 * Computes the ceiling function of an integer division.
 *
 * @param[in] A			- the dividend.
 * @param[in] B			- the divisor.
 */
#define CEIL(A, B)			(((A) - 1) / (B) + 1)

/**
 * Returns a bit mask to isolate the lowest part of a digit.
 *
 * @param[in] B			- the number of bits to isolate.
 */
#define MASK(B)				(((dig_t)1 << (B)) - 1)

/**
 * Returns a bit mask to isolate the lowest half of a digit.
 */
#define LMASK				(MASK(DIGIT >> 1))

/**
 * Returns a bit mask to isolate the highest half of a digit.
 */
#define HMASK				(LMASK << (DIGIT >> 1))

/**
 * Bit mask used to return an entire digit.
 */
#define DMASK				(HMASK | LMASK)

/**
 * Returns the lowest half of a digit.
 *
 * @param[in] D			- the digit.
 */
#define LOW(D)				(D & LMASK)

/**
 * Returns the highest half of a digit.
 *
 * @param[in] D			- the digit.
 */
#define HIGH(D)				(D >> (DIGIT >> 1))

/**
 * Returns the given character in upper case.
 *
 * @param[in] C			- the character.
 */
#define TOUPPER(C)			((C) - 0x20 * (((C) >= 'a') && ((C) <= 'z')))

/**
 * Renames the inline assembly macro to a prettier name.
 */
#define asm			__asm__

/**
 * Concatenates two tokens.
 */
/** @{ */
#define CAT(A, B)						_CAT(A, B)
#define _CAT(A, B)						A ## B
/** @} */

/**
 * Selects a basic version of an algorithm if no additional argument was passed.
 */
/** @{ */
#define ARGS(...)				_ARGS(__VA_ARGS__, _impl, _basic, _error)
#define _ARGS(...)				__ARGS(__VA_ARGS__)
#define __ARGS(_1, _2, N,...)	N
/** @} */

/**
 * Formats and prints data following a printf-like syntax.
 *
 * @param[in] ...				- the list of arguments matching the format.
 */
#ifndef QUIET
#define util_print(...)														\
	fprintf(stdout, __VA_ARGS__);											\
	fflush(stdout)
#else
#define util_print(...)			/* empty */
#endif

/**
 * Prints a standard label.
 *
 * @param[in] L					- the label of the banner.
 * @param[in] I					- if the banner is inside an hierarchy.
 */
#define util_print_banner(L, I)												\
	if (!I) {																\
		util_print("\n-- %s\n", L);											\
	} else {																\
		util_print("\n** %s\n\n", L);										\
	}																		\

/*============================================================================*/
/* Function prototypes                                                        */
/*============================================================================*/

/**
 * Convert a digit to big-endian.
 */
uint32_t util_conv_big(uint32_t i);

/**
 * Converts a small digit to a character.
 */
char util_conv_char(dig_t i);

/**
 * Returns the highest bit set on a digit.
 *
 * @param[in] a				- the digit.
 * @return the position of the highest bit set.
 */
int util_bits_dig(dig_t a);

#endif /* !RELIC_UTIL_H */
