/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2012 RELIC Authors
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
 * Interface of the low-level ternary field arithmetic module.
 *
 * All functions assume a configured polynomial basis f(z) and that the
 * destination has enough capacity to store the result of the computation.
 *
 * @version $Id$
 * @ingroup ft
 */

#ifndef RELIC_FT_LOW_H
#define RELIC_FT_LOW_H

/*============================================================================*/
/* Constant definitions                                                       */
/*============================================================================*/

#ifdef ASM

#include "relic_conf.h"

#undef FT_DIGS
#if (FT_POLYN % WORD) > 0
#define FT_DIGS	(FT_POLYN/WORD + 1)
#else
#define FT_DIGS	(FT_POLYN/WORD)
#endif

#else

#include "relic_types.h"

/*============================================================================*/
/* Function prototypes                                                        */
/*============================================================================*/

/**
 * Adds a digit vector and a digit. Computes c = a + digit.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first digit vector to add.
 * @param[in] digit			- the digit to add.
 */
void ft_add1_low(dig_t *c, dig_t *a, dig_t digit);

/**
 * Adds two digit vectors of the same size, with this size different than the
 * standard precision and specified in the last parameter. Computes c = a + b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first digit vector to add.
 * @param[in] b				- the second digit vector to add.
 * @param[in] digits		- the number of digits to add.
 * @param[in] size			- the size in digits of the operands.
 */
void ft_addd_low(dig_t *c, dig_t *a, dig_t *b, int digits, int size);

/**
 * Adds two digit vectors of the same size. Computes c = a + b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first digit vector to add.
 * @param[in] b				- the second digit vector to add.
 */
void ft_addn_low(dig_t *c, dig_t *a, dig_t *b);

/**
 * Subtracts a digit from a digit vector. Computes c = a - digit.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the digit vector to subtract from.
 * @param[in] digit			- the digit to subtract.
 */
void ft_sub1_low(dig_t *c, dig_t *a, dig_t digit);

/**
 * Subtracts two digit vectors of the same size, with this size different than
 * the standard precision and specified in the last parameter. Computes
 * c = a - b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the digit vector to subtract from.
 * @param[in] b				- the digit vector to subtract.
 * @param[in] digits		- the number of digits to subtract.
 * @param[in] size			- the size in digits of the operands.
 */
void ft_subd_low(dig_t *c, dig_t *a, dig_t *b, int digits, int size);

/**
 * Subtracts two digit vectors of the same size. Computes c = a 0 b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the digit vector to subtract from.
 * @param[in] b				- the digit vector to subtract.
 */
void ft_subn_low(dig_t *c, dig_t *a, dig_t *b);

/**
 * Compares two digits.
 *
 * @param[in] a				- the first digit to compare.
 * @param[in] b				- the second digit to compare.
 * @return FT_LT if a < b, FT_EQ if a == b and FT_GT if a > b.
 */
int ft_cmp1_low(dig_t a, dig_t b);

/**
 * Compares two digit vectors of the same size.
 *
 * @param[in] a				- the first digit vector to compare.
 * @param[in] b				- the second digit vector to compare.
 * @return FT_LT if a < b, FT_EQ if a == b and FT_GT if a > b.
 */
int ft_cmpn_low(dig_t *a, dig_t *b);

/**
 * Shifts a digit vector to the left by 1 bit. Computes c = a * z.
 *
 * @param[out] c			- the result
 * @param[in] a				- the digit vector to shift.
 * @return the carry of the last digit shift.
 */
dig_t ft_lsh1_low(dig_t *c, dig_t *a);

/**
 * Shifts a digit vector to the left by an amount smaller than a digit.
 * The shift amount must be bigger than 0 and smaller than FT_DIGIT. Computes
 * c = a * z^bits.
 *
 * @param[out] c			- the result
 * @param[in] a				- the digit vector to shift.
 * @param[in] bits			- the shift ammount.
 * @return the carry of the last digit shift.
 */
dig_t ft_lshb_low(dig_t *c, dig_t *a, int bits);

/**
 * Shifts a digit vector to the left by some digits.
 * Computes c = a * z^(digits * DIGIT).
 *
 * @param[out] c			- the result.
 * @param[in] a				- the multiple precision integer to shift.
 * @param[in] digits		- the shift amount.
 */
void ft_lshd_low(dig_t *c, dig_t *a, int digits);

/**
 * Shifts a digit vector to the right by 1 bit. Computes c = a / z.
 *
 * @param[out] c			- the result
 * @param[in] a				- the digit vector to shift.
 * @return the carry of the last digit shift.
 */
dig_t ft_rsh1_low(dig_t *c, dig_t *a);

/**
 * Shifts a digit vector to the right by an amount smaller than a digit.
 * The shift amount must be bigger than 0 and smaller than FT_DIGIT.
 * Computes c = a / (z^bits).
 *
 * @param[out] c			- the result
 * @param[in] a				- the digit vector to shift.
 * @param[in] bits			- the shift amount.
 * @return the carry of the last digit shift.
 */
dig_t ft_rshb_low(dig_t *c, dig_t *a, int bits);

/**
 * Shifts a digit vector to the right by some digits.
 * Computes c = a / z^(digits * DIGIT).
 *
 * @param[out] c			- the result.
 * @param[in] a				- the multiple precision integer to shift.
 * @param[in] digits		- the shift amount.
 */
void ft_rshd_low(dig_t *c, dig_t *a, int digits);

/**
 * Adds a left-shifted digit vector to another digit vector.
 * The shift amount must be shorter than the digit size.
 * Computes c = c + (a * z^bits).
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first digit vector to shift and add.
 * @param[in] size			- the number of digits to add.
 * @param[in] bits			- the shift amount.
 * @return the carry of the last shift.
 */
dig_t ft_lshadd_low(dig_t *c, dig_t *a, int bits, int size);

/**
 * Multiplies a digit vector by a digit.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the digit vector to multiply.
 * @param[in] digit			- the digit to multiply.
 * @return the most significant digit.
 */
void ft_mul1_low(dig_t *c, dig_t *a, dig_t digit);

/**
 * Multiplies two digit vectors of the same size.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first digit vector to multiply.
 * @param[in] b				- the second digit vector to multiply.
 */
void ft_muln_low(dig_t *c, dig_t *a, dig_t *b);

/**
 * Multiplies two digit vectors of the same size but smaller than the standard
 * precision.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first digit vector to multiply.
 * @param[in] b				- the second digit vector to multiply.
 * @param[in] size			- the size of the digit vectors.
 */
void ft_muld_low(dig_t *c, dig_t *a, dig_t *b, int size);

/**
 * Multiplies two digit vectors of the same size with embedded modular
 * reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first digit vector to multiply.
 * @param[in] b				- the second digit vector to multiply.
 */
void ft_mulm_low(dig_t *c, dig_t *a, dig_t *b);

/**
 * Cubes a digit vector using trit manipulation.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the digit vector to square.
 */
void ft_cubn_low(dig_t *c, dig_t *a);

/**
 * Cubes a digit vector using a lookup table.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the digit vector to square.
 */
void ft_cubl_low(dig_t *c, dig_t *a);

/**
 * Cubes a digit vector with embedded modular reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the digit vector to square.
 */
void ft_cubm_low(dig_t *c, dig_t *a);

/**
 * Extracts the square root of a digit vector.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the digit vector to extract the square root.
 */
void ft_crtn_low(dig_t *c, dig_t *a);

/**
 * Solves a quadratic equation for c^2 + c = a.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the digit vector.
 */
void ft_slvn_low(dig_t *c, dig_t *a);

/**
 * Reduces a double-precision digit vector modulo the configured irreducible
 * polynomial.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the digit vector to reduce.
 */
void ft_rdcm_low(dig_t *c, dig_t *a);

/**
 * Reduces a triple-precision digit vector modulo the configured irreducible
 * polynomial.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the digit vector to reduce.
 */
void ft_rdcc_low(dig_t *c, dig_t *a);

/**
 * Reduces the most significant bits of a digit vector modulo the configured
 * irreducible polynomial. The maximum number of bits to be reduced is equal
 * to the size of the digit.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the digit vector to reduce.
 */
void ft_rdc1_low(dig_t *c, dig_t *a);

#endif /* !ASM */

#endif /* !RELIC_FT_LOW_H */
