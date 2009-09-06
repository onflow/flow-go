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
 * Elementary types.
 *
 * @version $Id: relic_types.h 38 2009-06-03 18:15:41Z dfaranha $
 * @ingroup relic
 */

#ifndef RELIC_TYPES_H
#define RELIC_TYPES_H

#include <stdint.h>

#if ARITH == GMP
#include <gmp.h>
#endif

#include "relic_conf.h"

/*============================================================================*/
/* Constant definitions                                                       */
/*============================================================================*/

/**
 * Size in bits of a digit.
 */
#define DIGIT	(8 * sizeof(dig_t))

/**
 * Logarithm of the digit size in bits in base two.
 */
#if WORD == 8
#define DIGIT_LOG		3
#elif WORD == 16
#define DIGIT_LOG		4
#elif WORD == 32
#define DIGIT_LOG		5
#elif WORD == 64
#define DIGIT_LOG		6
#endif

/*============================================================================*/
/* Type definitions                                                           */
/*============================================================================*/

/**
 * Represents a digit from a multiple precision integer.
 *
 * Each digit is represented as an unsigned long to use the biggest native
 * type that potentially supports native instructions.
 */
#if ARITH == GMP
typedef mp_limb_t dig_t;
#elif WORD == 8
typedef uint8_t dig_t;
#elif WORD == 16
typedef uint16_t dig_t;
#elif WORD == 32
typedef uint32_t dig_t;
#elif WORD == 64
typedef uint64_t dig_t;
#endif

/**
 * Represents a signed digit.
 */
#if WORD == 8
typedef int8_t sig_t;
#elif WORD == 16
typedef int16_t sig_t;
#elif WORD == 32
typedef int32_t sig_t;
#elif WORD == 64
typedef int64_t sig_t;
#endif

/**
 * Represents a double-precision integer from a multiple precision integer.
 *
 * This is useful to store a result from a multiplication of two digits.
 */
#if WORD == 8
typedef uint16_t dbl_t;
#elif WORD == 16
typedef uint32_t dbl_t;
#elif WORD == 32
typedef uint64_t dbl_t;
#elif WORD == 64
#ifdef __GNUC__
typedef __uint128_t dbl_t;
#elif ARITH == EASY
#error "Easy backend in 64-bit mode supported only in GCC compiler."
#endif
#endif

/*============================================================================*/
/* Macro definitions                                                          */
/*============================================================================*/

/**
 * Specification for aligned variables.
 */
#if ALIGN > 1
#define align 			__attribute__ ((aligned (ALIGN)))
#else
#define align 			/* empty*/
#endif

/**
 * Align digit vector pointer to specified byte-boundary.
 *
 * @param[in,out] A		- the pointer to align.
 */
#if ALIGN > 1
#if ARCH == AVR || ARCH == MSP || ARCH == X86
#define ALIGNED(A)															\
	A = (dig_t *)((unsigned int)A + (ALIGN - ((unsigned int)A % ALIGN)));	\

#elif ARCH  == X86_64
#define ALIGNED(A)															\
	A = (dig_t *)((unsigned long)A + (ALIGN - ((unsigned long)A % ALIGN)));	\

#endif
#else
#define ALIGNED(...)	/* empty */
#endif

#endif /* !RELIC_TYPES_H */
