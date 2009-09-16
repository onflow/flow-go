/*
 * Copyright 2007 Project RELIC
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file.
 *
 * RELIC is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @defgroup fp2 Quadratic extension prime field arithmetic.
 */

/**
 * @file
 *
 * Interface of the quadratic extension prime field arithmetic functions.
 *
 * @version $Id$
 * @ingroup fp2
 */

#ifndef RELIC_FP2_H
#define RELIC_FP2_H

#include "relic_fp.h"
#include "relic_types.h"

/**
 * Represents a quadratic extension prime field element.
 */
typedef fp_t fp2_t[2];

/**
 * Represents a prime field element with automatic memory allocation.
 */
typedef fp_st fp2_st[2];

/**
 * Allocate and initializes a quadratic extension prime field element.
 *
 * @param[out] A			- the new quadratic extension field element.
 */
#define fp2_new(A)															\
		fp_new(A[0]); fp_new(A[1]);											\

/**
 * Calls a function to clean and free a quadratic extension field element.
 *
 * @param[out] A			- the quadratic extension field element to free.
 */
#define fp2_free(A)															\
		fp_free(A[0]); fp_free(A[1]); 										\

/**
 * Copies the second argument to the first argument.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the quadratic extension field element to copy.
 */
#define fp2_copy(C, A)														\
		fp_copy(C[0], A[0]); fp_copy(C[1], A[1]); 							\

/**
 * Negates a quadratic extension field element.
 *
 * @param[out] c			- the result.
 * @param[out] a			- the quadratic extension field element to negate.
 */
#define fp2_neg(C, A)														\
		fp_neg(C[0], A[0]); fp_neg(C[1], A[1]); 							\

/**
 * Assigns zero to a quadratic extension field element.
 *
 * @param[out] a			- the quadratic extension field element to zero.
 */
#define fp2_zero(A)															\
		fp_zero(A[0]); fp_zero(A[1]); 										\

#define fp2_set_dig(A, B)													\
		fp_set_dig(A[0], B); fp_zero(A[1]);									\

/**
 * Tests if a quadratic extension field element is zero or not.
 *
 * @param[in] a				- the prime field element to compare.
 * @return 1 if the argument is zero, 0 otherwise.
 */
#define fp2_is_zero(A)														\
		fp_is_zero(A[0]) && fp_is_zero(A[1]) 								\

/**
 * Assigns a random value to a quadratic extension field element.
 *
 * @param[out] a			- the prime field element to assign.
 */
#define fp2_rand(A)															\
		fp_rand(A[0]); fp_rand(A[1]);										\

/**
 * Prints a quadratic extension field element to standard output.
 *
 * @param[in] a				- the quadratic extension field element to print.
 */
#define fp2_print(A)														\
		fp_print(A[0]); fp_print(A[1]);										\

/**
 * Reads a quadratic extension field element from strings in a given radix.
 * The radix must be a power of 2 included in the interval [2, 64].
 *
 * @param[out] a			- the result.
 * @param[in] str0			- 
 * @param[in] str1			- 
 * @param[in] len 			- the maximum length of the strings.
 * @param[in] radix			- the radix.
 * @throw ERR_INVALID		- if the radix is invalid.
 */
#define fp2_read(A, STR0, STR1, LEN, RADIX)									\
		fp_read(A[0], STR0, LEN, RADIX);									\
		fp_read(A[1], STR1, LEN, RADIX);									\

/**
 * Writes a quadratic extension field element to strings in a given radix.
 * The radix must be a power of 2 included in the interval [2, 64].
 *
 * @param[out] str0			- 
 * @param[out] str1			- 
 * @param[in] len			- the buffer capacities.
 * @param[in] a				- the prime field element to write.
 * @param[in] radix			- the radix.
 * @throw ERR_BUFFER		- if the buffer capacity is insufficient.
 * @throw ERR_INVALID		- if the radix is invalid.
 */
#define fp2_write(STR0, STR1, LEN, A, RADIX)								\
		fp_write(STR0, LEN, A[0], RADIX);										\
		fp_write(STR1, LEN, A[1], RADIX);										\

/**
 * Returns the result of a comparison between two quadratic extension field
 * elements
 *
 * @param[in] a				- the first quadratic extension field element.
 * @param[in] b				- the second quadratic extension field element.
 * @return FP_LT if a < b, FP_EQ if a == b and FP_GT if a > b.
 */
#define fp2_cmp(A, B)														\
		((fp_cmp(A[0], B[0]) == CMP_EQ) && (fp_cmp(A[1], B[1]) == CMP_EQ)	\
		? CMP_EQ : CMP_NE)													\

#define fp2_set_dig(A, B)													\
		fp_set_dig(A[0], B); fp_zero(A[1]);									\

/**
 * Adds two quadratic extension field elements, that is, computes c = a + b.
 *
 * @param[out] c			- the destination.
 * @param[in] a				- the first quadratic extension field element.
 * @param[in] b				- the second quadratic extension field element.
 */
#define fp2_add(C, A, B)													\
		fp_add(C[0], A[0], B[0]); fp_add(C[1], A[1], B[1]);					\

/**
 * Subtracts a prime field element from another, that is, computes
 * c = a - b.
 *
 * @param[out] c			- the destination.
 * @param[in] a				- the quadratic extension prime field element.
 * @param[in] b				- the quadratic extension prime field element.
 */
#define fp2_sub(C, A, B)													\
		fp_sub(C[0], A[0], B[0]); fp_sub(C[1], A[1], B[1]);					\

/**
 * Adds two quadratic extension field elements, that is, computes c = a + b.
 *
 * @param[out] c			- the destination.
 * @param[in] a				- the first quadratic extension field element.
 * @param[in] b				- the second quadratic extension field element.
 */
#define fp2_dbl(C, A)														\
		fp_dbl(C[0], A[0]); fp_dbl(C[1], A[1]);								\

#define fp2_add_dig(C, A, B)												\
		fp_add_dig(C[0], A[0], B); fp_copy(C[1], A[1]);						\

#define fp2_sub_dig(C, A, B)												\
		fp_sub_dig(C[0], A[0], B); fp_copy(C[1], A[1]);						\

/**
 * Multiples two prime field elements, that is, compute c = a * b.
 *
 * @param[out] c			- the destination.
 * @param[in] a				- the quadratic extension prime field element.
 * @param[in] b				- the quadratic extension prime field element.
 */
void fp2_mul(fp2_t c, fp2_t a, fp2_t b);

/**
 * Computes the square of a prime field element, that is, computes
 * c = a * a.
 *
 * @param[out] c			- the destination.
 * @param[in] a				- the prime field element to square.
 */
void fp2_sqr(fp2_t c, fp2_t a);

/**
 *
 *
 */
void fp2_inv(fp2_t c, fp2_t a);

void fp2_conj(fp2_t c, fp2_t a);

void fp2_mul_poly(fp2_t c, fp2_t a);

#endif /* !RELIC_FP2_H */
