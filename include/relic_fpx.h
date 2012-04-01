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
 * @defgroup fpx Extensions of prime fields.
 */

/**
 * @file
 *
 * Interface of the module for extension field arithmetic over prime fields.
 *
 * @version $Id$
 * @ingroup fpx
 */

#ifndef RELIC_FPX_H
#define RELIC_FPX_H

#include "relic_fp.h"
#include "relic_types.h"

/*============================================================================*/
/* Type definitions                                                           */
/*============================================================================*/

/**
 * Represents a quadratic extension prime field element.
 *
 * This extension is constructed with the basis {1, i}, where i is an
 * adjoined square root in the prime field. If p = 3 mod 8, u = i, i^2 = -1;
 * otherwise i^2 = -2.
 */
typedef fp_t fp2_t[2];

/**
 * Represents a double-precision quadratic extension field element.
 */
typedef dv_t dv2_t[2];

/**
 * Represents a quadratic extension field element with automatic memory
 * allocation.
 */
typedef fp_st fp2_st[2];

/**
 * Represents a dodecic extension field element.
 *
 * This extension is constructed with the basis {1, v, v^2}, where v^3 = E is an
 * adjoined cube root in the underlying quadratic extension.  If p = 3 mod 8,
 * E = u + 1; if p = 5 mod 8, E = u; if p = 7 mod 8 and p = 2,3 mod 5,
 * E = u + 2.
 */
typedef fp2_t fp6_t[3];

/**
 * Represents a double-precision dodecic extension field element.
 */
typedef dv2_t dv6_t[3];

/**
 * Represents a dodecic extension field element.
 *
 * This extension is constructed with the basis {1, w}, where w^2 = v is an
 * adjoined square root in the underlying dodecic extension.
 */
typedef fp6_t fp12_t[2];

/*============================================================================*/
/* Macro definitions                                                          */
/*============================================================================*/

/**
 * Initializes a quadratic extension field element with a null value.
 *
* @param[out] A			- the quadratic extension element to initialize.
 */
#define fp2_null(A)															\
		fp_null(A[0]); fp_null(A[1]);										\

/**
 * Initializes a double-precision quadratic extension field element with a null
 * value.
 *
* @param[out] A			- the quadratic extension element to initialize.
 */
#define dv2_null(A)															\
		dv_null(A[0]); dv_null(A[1]);										\

/**
 * Allocate and initializes a quadratic extension field element.
 *
 * @param[out] A			- the new quadratic extension field element.
 */
#define fp2_new(A)															\
		fp_new(A[0]); fp_new(A[1]);											\

/**
 * Allocate and initializes a double-precision quadratic extension field element.
 *
 * @param[out] A			- the new quadratic extension field element.
 */
#define dv2_new(A)															\
		dv_new(A[0]); dv_new(A[1]);											\

/**
 * Calls a function to clean and free a quadratic extension field element.
 *
 * @param[out] A			- the quadratic extension field element to free.
 */
#define fp2_free(A)															\
		fp_free(A[0]); fp_free(A[1]); 										\

/**
 * Calls a function to clean and free a double-precision quadratic extension
 * field element.
 *
 * @param[out] A			- the quadratic extension field element to free.
 */
#define dv2_free(A)															\
		dv_free(A[0]); dv_free(A[1]); 										\

/**
 * Adds two quadratic extension field elements. Computes C = A + B.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the first quadratic extension field element.
 * @param[in] B				- the second quadratic extension field element.
 */
#if PP_QDR == BASIC
#define fp2_add(C, A, B)	fp2_add_basic(C, A, B)
#elif PP_QDR == INTEG
#define fp2_add(C, A, B)	fp2_add_integ(C, A, B)
#endif

/**
 * Subtracts a quadratic extension field elements from another.
 * Computes C = A - B.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the first quadratic extension field element.
 * @param[in] B				- the second quadratic extension field element.
 */
#if PP_QDR == BASIC
#define fp2_sub(C, A, B)	fp2_sub_basic(C, A, B)
#elif PP_QDR == INTEG
#define fp2_sub(C, A, B)	fp2_sub_integ(C, A, B)
#endif

/**
 * Doubles a quadratic extension field elements. Computes C = A + A.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the quadratic extension field element.
 */
#if PP_QDR == BASIC
#define fp2_dbl(C, A)		fp2_dbl_basic(C, A)
#elif PP_QDR == INTEG
#define fp2_dbl(C, A)		fp2_dbl_integ(C, A)
#endif

/**
 * Multiplies two quadratic extension field elements. Computes C = A * B.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the first quadratic extension field element.
 * @param[in] B				- the second quadratic extension field element.
 */
#if PP_QDR == BASIC
#define fp2_mul(C, A, B)	fp2_mul_basic(C, A, B)
#elif PP_QDR == INTEG
#define fp2_mul(C, A, B)	fp2_mul_integ(C, A, B)
#endif

/**
 * Multiplies a quadratic extension field by the quadratic/cubic non-residue.
 * Computes C = A * E, where E is a non-square/non-cube in the quadratic
 * extension.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the quadratic extension field element to multiply.
 */
#if PP_QDR == BASIC
#define fp2_mul_nor(C, A)	fp2_mul_nor_basic(C, A)
#elif PP_QDR == INTEG
#define fp2_mul_nor(C, A)	fp2_mul_nor_integ(C, A)
#endif

/**
 * Squares a quadratic extension field elements. Computes C = A * A.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the quadratic extension field element to square.
 */
#if PP_QDR == BASIC
#define fp2_sqr(C, A)		fp2_sqr_basic(C, A)
#elif PP_QDR == INTEG
#define fp2_sqr(C, A)		fp2_sqr_integ(C, A)
#endif

/**
 * Initializes a dodecic extension field with a null value.
 *
 * @param[out] A			- the dodecic extension element to initialize.
 */
#define fp6_null(A)															\
		fp2_null(A[0]); fp2_null(A[1]); fp2_null(A[2]);						\

/**
 * Initializes a double-precision dodecic extension field with a null value.
 *
 * @param[out] A			- the dodecic extension element to initialize.
 */
#define dv6_null(A)															\
		dv2_null(A[0]); dv2_null(A[1]); dv2_null(A[2]);						\

/**
 * Allocate and initializes a dodecic extension field element.
 *
 * @param[out] A			- the new dodecic extension field element.
 */
#define fp6_new(A)															\
		fp2_new(A[0]); fp2_new(A[1]); fp2_new(A[2]);						\

/**
 * Calls a function to clean and free a dodecic extension field element.
 *
 * @param[out] A			- the dodecic extension field element to free.
 */
#define fp6_free(A)															\
		fp2_free(A[0]); fp2_free(A[1]); fp2_free(A[2]); 					\

/**
 * Allocate and initializes a double-precision dodecic extension field element.
 *
 * @param[out] A			- the new dodecic extension field element.
 */
#define dv6_new(A)															\
		dv2_new(A[0]); dv2_new(A[1]); dv2_new(A[2]);						\

/**
 * Calls a function to clean and free a double-precision dodecic extension field
 * element.
 *
 * @param[out] A			- the dodecic extension field element to free.
 */
#define dv6_free(A)															\
		dv2_free(A[0]); dv2_free(A[1]); dv2_free(A[2]); 					\

/**
 * Multiplies two sextic extension field elements. Computes C = A * B.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the first dodecic extension field element.
 * @param[in] B				- the second dodecic extension field element.
 */
#if PP_EXT == BASIC
#define fp6_mul(C, A, B)	fp6_mul_basic(C, A, B)
#elif PP_EXT == LAZYR
#define fp6_mul(C, A, B)	fp6_mul_lazyr(C, A, B)
#endif

/**
 * Squares a sextic extension field element. Computes C = A * A.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the dodecic extension field element to square.
 */
#if PP_EXT == BASIC
#define fp6_sqr(C, A)		fp6_sqr_basic(C, A)
#elif PP_EXT == LAZYR
#define fp6_sqr(C, A)		fp6_sqr_lazyr(C, A)
#endif

/**
 * Initializes a dodecic extension field with a null value.
 *
 * @param[out] A			- the dodecic extension element to initialize.
 */
#define fp12_null(A)														\
		fp6_null(A[0]); fp6_null(A[1]);										\

/**
 * Allocate and initializes a dodecic extension field element.
 *
 * @param[out] A			- the new dodecic extension field element.
 */
#define fp12_new(A)															\
		fp6_new(A[0]); fp6_new(A[1]);										\

/**
 * Calls a function to clean and free a dodecic extension field element.
 *
 * @param[out] A			- the dodecic extension field element to free.
 */
#define fp12_free(A)														\
		fp6_free(A[0]); fp6_free(A[1]); 									\

/**
 * Multiplies two dodecic extension field elements. Computes C = A * B.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the first dodecic extension field element.
 * @param[in] B				- the second dodecic extension field element.
 */
#if PP_EXT == BASIC
#define fp12_mul(C, A, B)		fp12_mul_basic(C, A, B)
#elif PP_EXT == LAZYR
#define fp12_mul(C, A, B)		fp12_mul_lazyr(C, A, B)
#endif

/**
 * Multiplies a dense and a sparse dodecic extension field elements. Computes
 * C = A * B.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the dense dodecic extension field element.
 * @param[in] B				- the sparse dodecic extension field element.
 */
#if PP_EXT == BASIC
#define fp12_mul_dxs(C, A, B)	fp12_mul_dxs_basic(C, A, B)
#elif PP_EXT == LAZYR
#define fp12_mul_dxs(C, A, B)	fp12_mul_dxs_lazyr(C, A, B)
#endif

/**
 * Squares a dodecic extension field elements in the cyclotomic subgroup.
 * Computes C = A * B.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the dodecic extension field element to square.
 */
#if PP_EXT == BASIC
#define fp12_sqr_cyc(C, A)		fp12_sqr_cyc_basic(C, A)
#elif PP_EXT == LAZYR
#define fp12_sqr_cyc(C, A)		fp12_sqr_cyc_lazyr(C, A)
#endif

/**
 * Squares a dodecic extension field elements in the cyclotomic subgroup in.
 * compressed form. Computes C = A * B.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the dodecic extension field element to square.
 */
#if PP_EXT == BASIC
#define fp12_sqr_pck(C, A)		fp12_sqr_pck_basic(C, A)
#elif PP_EXT == LAZYR
#define fp12_sqr_pck(C, A)		fp12_sqr_pck_lazyr(C, A)
#endif

/*============================================================================*/
/* Function prototypes                                                        */
/*============================================================================*/

/**
 * Computes the constant needed to compute the Frobenius map.
 *
 * @param[out] f			- the result.
 */
void fp2_const_calc(void);

/**
 * Copies the second argument to the first argument.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the quadratic extension field element to copy.
 */
void fp2_copy(fp2_t c, fp2_t a);

/**
 * Assigns zero to a quadratic extension field element.
 *
 * @param[out] A			- the quadratic extension field element to zero.
 */
void fp2_zero(fp2_t a);

/**
 * Tests if a quadratic extension field element is zero or not.
 *
 * @param[in] A				- the quadratic extension field element to compare.
 * @return 1 if the argument is zero, 0 otherwise.
 */
int fp2_is_zero(fp2_t a);

/**
 * Assigns a random value to a quadratic extension field element.
 *
 * @param[out] A			- the quadratic extension field element to assign.
 */
void fp2_rand(fp2_t a);

/**
 * Prints a quadratic extension field element to standard output.
 *
 * @param[in] A				- the quadratic extension field element to print.
 */
void fp2_print(fp2_t a);

/**
 * Returns the result of a comparison between two quadratic extension field
 * elements
 *
 * @param[in] A				- the first quadratic extension field element.
 * @param[in] B				- the second quadratic extension field element.
 * @return CMP_LT if a < b, CMP_EQ if a == b and CMP_GT if a > b.
 */
int fp2_cmp(fp2_t a, fp2_t b);

/**
 * Adds two quadratic extension field elements using basic arithmetic.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first quadratic extension field element.
 * @param[in] b				- the second quadratic extension field element.
 */
void fp2_add_basic(fp2_t c, fp2_t a, fp2_t b);

/**
 * Adds two quadratic extension field elements using lower-level implementation.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first quadratic extension field element.
 * @param[in] b				- the second quadratic extension field element.
 */
void fp2_add_integ(fp2_t c, fp2_t a, fp2_t b);

/**
 * Subtracts a quadratic extension field element from another using basic
 * arithmetic.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the first quadratic extension field element.
 * @param[in] B				- the second quadratic extension field element.
 */
void fp2_sub_basic(fp2_t c, fp2_t a, fp2_t b);

/**
 * Subtracts a quadratic extension field element from another using lower-level
 * implementation.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the first quadratic extension field element.
 * @param[in] B				- the second quadratic extension field element.
 */
void fp2_sub_integ(fp2_t c, fp2_t a, fp2_t b);

/**
 * Negates a quadratic extension field element.
 *
 * @param[out] C			- the result.
 * @param[out] A			- the quadratic extension field element to negate.
 */
void fp2_neg(fp2_t c, fp2_t a);

/**
 * Doubles a quadratic extension field element.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension field element to double.
 */
void fp2_dbl_basic(fp2_t c, fp2_t a);

/**
 * Doubles a quadratic extension field element.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension field element to double.
 */
void fp2_dbl_integ(fp2_t c, fp2_t a);

/**
 * Multiples two quadratic extension field elements using basic arithmetic.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first quadratic extension field element.
 * @param[in] b				- the second quadratic extension field element.
 */
void fp2_mul_basic(fp2_t c, fp2_t a, fp2_t b);

/**
 * Multiples two quadratic extension field elements using lower-level
 * implementation.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first quadratic extension field element.
 * @param[in] b				- the second quadratic extension field element.
 */
void fp2_mul_integ(fp2_t c, fp2_t a, fp2_t b);

/**
 * Multiplies a quadratic extension field element by the adjoined square root.
 * Computes c = a * u.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension field element to multiply.
 */
void fp2_mul_art(fp2_t c, fp2_t a);

/**
 * Multiplies a quadratic extension field element by a quadratic/cubic
 * non-residue.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension field element to multiply.
 */
void fp2_mul_nor_basic(fp2_t c, fp2_t a);

/**
 * Multiplies a quadratic extension field element by a quadratic/cubic
 * non-residue using integrated reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension field element to multiply.
 */
void fp2_mul_nor_integ(fp2_t c, fp2_t a);

/**
 * Multiplies a quadratic extension field element by a power of the constant
 * needed to compute a power of the Frobenius map.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the field element to multiply.
 * @param[in] i				- the power of the Frobenius map.
 * @param[in] j				- the power of the constant.
 */
void fp2_mul_frb(fp2_t c, fp2_t a, int i, int j);

/**
 * Multiplies a quadratic extension field element by a power of the constant
 * needed to compute the repeated Frobenius map.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the field element to multiply.
 * @param[in] i				- the power of the constant.
 */
void fp2_mul_frb_sqr(fp2_t c, fp2_t a, int i);

/**
 * Computes the square of a quadratic extension field element using basic
 * arithmetic.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension field element to square.
 */
void fp2_sqr_basic(fp2_t c, fp2_t a);

/**
 * Computes the square of a quadratic extension field element using the lower-
 * level implementation.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension field element to square.
 */
void fp2_sqr_integ(fp2_t c, fp2_t a);

/**
 * Inverts multiple quadratic extension field elements simultaneously.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension field elements to invert.
 * @param[in] n				- the number of elements.
 */
void fp2_inv_sim(fp2_t *c, fp2_t *a, int n);

/**
 * Inverts a quadratic extension field element. Computes c = a^(-1).
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension field element to invert.
 */
void fp2_inv(fp2_t c, fp2_t a);

/**
 * Computes a power of a quadratic extension field element.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension element to exponentiate.
 * @param[in] b				- the exponent.
 */
void fp2_exp(fp2_t c, fp2_t a, bn_t b);

/**
 * Computes a power of the Frobenius map of a quadratic extension field element.
 * When i is odd, his is the same as computing the conjugate of the extension
 * field element.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension element to conjugate.
 * @param[in] i				- the power of the Frobenius map.
 */
void fp2_frb(fp2_t c, fp2_t a, int i);

/**
 * Extracts the square root of a quadratic extension field element.
 * Computes c = sqrt(a). The other square root is the negation of c.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the prime field element.
 * @return					- 1 if there is a square root, 0 otherwise.
 */
int fp2_srt(fp2_t c, fp2_t a);

/**
 * Copies the second argument to the first argument.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the sextic extension field element to copy.
 */
void fp6_copy(fp6_t c, fp6_t a);

/**
 * Assigns zero to a sextic extension field element.
 *
 * @param[out] A			- the sextic extension field element to zero.
 */
void fp6_zero(fp6_t a);

/**
 * Tests if a sextic extension field element is zero or not.
 *
 * @param[in] A				- the sextic extension field element to compare.
 * @return 1 if the argument is zero, 0 otherwise.
 */
int fp6_is_zero(fp6_t a);

/**
 * Assigns a random value to a sextic extension field element.
 *
 * @param[out] A			- the sextic extension field element to assign.
 */
void fp6_rand(fp6_t a);

/**
 * Prints a sextic extension field element to standard output.
 *
 * @param[in] A				- the sextic extension field element to print.
 */
void fp6_print(fp6_t a);

/**
 * Returns the result of a comparison between two sextic extension field
 * elements
 *
 * @param[in] A				- the first sextic extension field element.
 * @param[in] B				- the second sextic extension field element.
 * @return CMP_LT if a < b, CMP_EQ if a == b and CMP_GT if a > b.
 */
int fp6_cmp(fp6_t a, fp6_t b);

/**
 * Adds two sextic extension field elements. Computes C = A + B.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the first sextic extension field element.
 * @param[in] B				- the second sextic extension field element.
 */
void fp6_add(fp6_t c, fp6_t a, fp6_t b);

/**
 * Subtracts a sextic extension field element from another. Computes
 * C = A - B.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the sextic extension field element.
 * @param[in] B				- the sextic extension field element.
 */
void fp6_sub(fp6_t c, fp6_t a, fp6_t b);

/**
 * Negates a sextic extension field element.
 *
 * @param[out] C			- the result.
 * @param[out] A			- the sextic extension field element to negate.
 */
void fp6_neg(fp6_t c, fp6_t a);

/**
 * Doubles a sextic extension field element. Computes c = 2 * a.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the sextic extension field element to double.
 */
void fp6_dbl(fp6_t c, fp6_t a);

/**
 * Multiples two sextic extension field elements without performing modular
 * reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the sextic extension field element.
 * @param[in] b				- the sextic extension field element.
 */
void fp6_mul_unr(dv6_t c, fp6_t a, fp6_t b);

/**
 * Multiples two sextic extension field elements.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the sextic extension field element.
 * @param[in] b				- the sextic extension field element.
 */
void fp6_mul_basic(fp6_t c, fp6_t a, fp6_t b);

/**
 * Multiples two sextic extension field elements using lazy reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the sextic extension field element.
 * @param[in] b				- the sextic extension field element.
 */
void fp6_mul_lazyr(fp6_t c, fp6_t a, fp6_t b);

/**
 * Multiplies a sextic extension field element by the adjoined cube root.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the sextic extension field element to multiply.
 */
void fp6_mul_art(fp6_t c, fp6_t a);

/**
 * Multiples a sextic extension field element by a quadratic extension
 * field element.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the sextic extension field element.
 * @param[in] b				- the quadratic extension field element.
 */
void fp6_mul_dxq(fp6_t c, fp6_t a, fp2_t b);

/**
 * Multiples a dense sextic extension field element by a sparse element.
 *
 * The sparse element must have a[2] = 0.
 *
 * @param[out] c			- the result.
 * @param[in] a				- a sextic extension field element.
 * @param[in] b				- a sparse sextic extension field element.
 */
void fp6_mul_dxs(fp6_t c, fp6_t a, fp6_t b);

/**
 * Squares a sextic extension field element.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the sextic extension field element to square.
 */
void fp6_sqr_basic(fp6_t c, fp6_t a);

/**
 * Squares a sextic extension field element using lazy reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the sextic extension field element to square.
 */
void fp6_sqr_lazyr(fp6_t c, fp6_t a);

/**
 * Inverts a sextic extension field element. Computes c = a^(-1).
 *
 * @param[out] c			- the result.
 * @param[in] a				- the sextic extension field element to invert.
 */
void fp6_inv(fp6_t c, fp6_t a);

/**
 * Computes a power of a sextic extension field element. Computes c = a^b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the sextic extension element to exponentiate.
 * @param[in] b				- the exponent.
 */
void fp6_exp(fp6_t c, fp6_t a, bn_t b);

/**
 * Computes a power of the Frobenius endomorphism of a sextic extension field
 * element. Computes c = a^p^i.
 *
 * @param[out] c			- the result.
 * @param[in] a				- a sextic extension field element.
 * @param[in] i				- the power of the Frobenius map.
 */
void fp6_frb(fp6_t c, fp6_t a, int i);

/**
* Computes two consecutive Frobenius endomorphisms of a sextic extension
* element. Computes c = a^(p^2).
 *
 * @param[out] c			- the result.
 * @param[in] a				- a sextic extension field element.
 */
void fp6_frb_sqr(fp6_t c, fp6_t a);

/**
 * Copies the second argument to the first argument.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the dodecic extension field element to copy.
 */
void fp12_copy(fp12_t c, fp12_t a);

/**
 * Assigns zero to a dodecic extension field element.
 *
 * @param[out] A			- the dodecic extension field element to zero.
 */
void fp12_zero(fp12_t a);

/**
 * Tests if a dodecic extension field element is zero or not.
 *
 * @param[in] A				- the dodecic extension field element to compare.
 * @return 1 if the argument is zero, 0 otherwise.
 */
int fp12_is_zero(fp12_t a);

/**
 * Assigns a random value to a dodecic extension field element.
 *
 * @param[out] A			- the dodecic extension field element to assign.
 */
void fp12_rand(fp12_t a);

/**
 * Prints a dodecic extension field element to standard output.
 *
 * @param[in] A				- the dodecic extension field element to print.
 */
void fp12_print(fp12_t a);

/**
 * Returns the result of a comparison between two dodecic extension field
 * elements
 *
 * @param[in] a				- the first dodecic extension field element.
 * @param[in] b				- the second dodecic extension field element.
 * @return CMP_LT if a < b, CMP_EQ if a == b and CMP_GT if a > b.
 */
int fp12_cmp(fp12_t a, fp12_t b);

/**
 * Returns the result of a signed comparison between a dodecic extension field
 * element and a digit.
 *
 * @param[in] a				- the dodecic extension field element.
 * @param[in] b				- the digit.
 * @return CMP_LT if a < b, CMP_EQ if a == b and CMP_GT if a > b.
 */
int fp12_cmp_dig(fp12_t a, dig_t b);

/**
 * Assigns a dodecic extension field element to a digit.
 *
 * @param[in] a				- the dodecic extension field element.
 * @param[in] b				- the digit.
 */
void fp12_set_dig(fp12_t a, dig_t b);

/**
 * Adds two dodecic extension field elements. Computes C = A + B.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the first dodecic extension field element.
 * @param[in] B				- the second dodecic extension field element.
 */
void fp12_add(fp12_t c, fp12_t a, fp12_t b);

/**
 * Subtracts a dodecic extension field element from another. Computes
 * C = A - B.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the first dodecic extension field element.
 * @param[in] B				- the second dodecic extension field element.
 */
void fp12_sub(fp12_t c, fp12_t a, fp12_t b);

/**
 * Negates a dodecic extension field element.
 *
 * @param[out] C			- the result.
 * @param[out] A			- the dodecic extension field element to negate.
 */
void fp12_neg(fp12_t c, fp12_t a);

/**
 * Multiples two dodecic extension field elements without performing modular
 * reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the dodecic extension field element.
 * @param[in] b				- the dodecic extension field element.
 */
void fp12_mul_unr(dv6_t c, fp12_t a, fp12_t b);

/**
 * Multiples two dodecic extension field elements.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the dodecic extension field element.
 * @param[in] b				- the dodecic extension field element.
 */
void fp12_mul_basic(fp12_t c, fp12_t a, fp12_t b);

/**
 * Multiples two dodecic extension field elements using lazy reduction.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the dodecic extension field element.
 * @param[in] b				- the dodecic extension field element.
 */
void fp12_mul_lazyr(fp12_t c, fp12_t a, fp12_t b);

/**
 * Multiples a dense dodecic extension field element by a sparse element.
 *
 * The sparse element must have only the a[0][0], a[1][0] and a[1][1] elements
 * not equal to zero.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the dense dodecic extension field element.
 * @param[in] b				- the sparse dodecic extension field element.
 */
void fp12_mul_dxs(fp12_t c, fp12_t a, fp12_t b);

/**
 * Computes the square of a dodecic extension field element. Computes
 * c = a * a.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the dodecic extension field element to square.
 */
void fp12_sqr(fp12_t c, fp12_t a);

/**
 * Computes the doubled square of a dodecic extension field element. Computes
 * c = 2 * a * a.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the dodecic extension field element to double
 * 							  square.
 */
void fp12_sqr_dbl(fp12_t c, fp12_t a);

/**
 * Computes the square of a cyclotomic dodecic extension field element.
 *
 * A cyclotomic element is one raised to the (p^6 - 1)(p^2 + 1)-th power.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the cyclotomic extension element to square.
 */
void fp12_sqr_cyc_basic(fp12_t c, fp12_t a);

/**
 * Computes the square of a cyclotomic dodecic extension field element using
 * lazy reduction.
 *
 * A cyclotomic element is one raised to the (p^6 - 1)(p^2 + 1)-th power.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the cyclotomic extension element to square.
 */
void fp12_sqr_cyc_lazyr(fp12_t c, fp12_t a);

/**
 * Computes the square of a compressed cyclotomic extension field element.
 *
 * A cyclotomic element is one raised to the (p^6 - 1)(p^2 + 1)-th power.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the cyclotomic extension element to square.
 */
void fp12_sqr_pck_basic(fp12_t c, fp12_t a);

/**
 * Computes the square of a compressed cyclotomic extension field element using
 * lazy reduction.
 *
 * A cyclotomic element is one raised to the (p^6 - 1)(p^2 + 1)-th power.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the cyclotomic extension element to square.
 */
void fp12_sqr_pck_lazyr(fp12_t c, fp12_t a);

/**
 * Tests if a dodecic extension field element belongs to the cyclotomic
 * subgroup.
 *
 * @param[in] a				- the dodecic extension field element to test.
 * @return 1 if the extension field element is in the subgroup. 0 otherwise.
 */
int fp12_test_cyc(fp12_t a);

/**
 * Converts a dodecic extension field element to a cyclotomic element.
 * Computes c = a^(p^12 - 1)*(p^2 + 1).
 *
 * @param[out] c			- the result.
 * @param[in] a				- a dodecic extension field element.
 */
void fp12_conv_cyc(fp12_t c, fp12_t a);

/**
 * Decompresses a compressed cyclotomic extension field element to its
 * usual representation.
 *
 * @param[out] c			- the result.
 * @param[in] a				- a dodecic extension field element to decompress.
 */
void fp12_back_cyc(fp12_t c, fp12_t a);

/**
 * Decompresses multiple compressed cyclotomic extension field elements to its
 * usual representation.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the dodecic field elements to decompress.
 * @param[in] n				- the number of field elements to decompress.
 */
void fp12_back_cyc_sim(fp12_t *c, fp12_t *a, int n);

/**
 * Inverts a dodecic extension field element. Computes c = a^(-1).
 *
 * @param[out] c			- the result.
 * @param[in] a				- the dodecic extension field element to invert.
 */
void fp12_inv(fp12_t c, fp12_t a);

/**
 * Computes the inverse of a unitary dodecic extension field element.
 *
 * For unitary elements, this is equivalent to computing the conjugate.
 * A unitary element is one previously raised to the (p^6 - 1)-th power.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the dodecic extension field element to invert.
 */
void fp12_inv_uni(fp12_t c, fp12_t a);

/**
 * Converts a dodecic extension field element to a unitary element.
 * Computes c = a^(p^12 - 1).
 *
 * @param[out] c			- the result.
 * @param[in] a				- a dodecic extension field element.
 */
void fp12_conv_uni(fp12_t c, fp12_t a);

/**
 * Computes the Frobenius endomorphism of a dodecic extension element.
 * Computes c = a^p.
 *
 * @param[out] c			- the result.
 * @param[in] a				- a dodecic extension field element.
 * @param[in] i				- the power of the Frobenius map.
 */
void fp12_frb(fp12_t c, fp12_t a, int i);

/**
 * Computes two consecutive Frobenius endomorphisms of a dodecic extension
 * element. Computes c = a^(p^2).
 *
 * @param[out] c			- the result.
 * @param[in] a				- a dodecic extension field element.
 */
void fp12_frb_sqr(fp12_t c, fp12_t a);

/**
 * Computes a power of a dodecic extension field element. Detects if the
 * extension field element is in a cyclotomic subgroup and if this is the case,
 * a faster squaring is used.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the basis.
 * @param[in] b				- the exponent.
 */
void fp12_exp(fp12_t c, fp12_t a, bn_t b);

/**
 * Computes a power of a cyclotomic dodecic extension field element.
 *
 * A cyclotomic element is one previously raised to the (p^6 - 1)-th power.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the basis.
 * @param[in] b				- the exponent.
 */
void fp12_exp_cyc(fp12_t c, fp12_t a, bn_t b);

/**
 * Computes a power of a cyclotomic dodecic extension field element.
 *
 * A cyclotomic element is one previously raised to the (p^6 - 1)-th power.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the basis.
 * @param[in] b				- the exponent in sparse form.
 * @param[in] l				- the length of the exponent in sparse form.
 */
void fp12_exp_cyc_sps(fp12_t c, fp12_t a, int *b, int l);

#endif /* !RELIC_FPX_H */
