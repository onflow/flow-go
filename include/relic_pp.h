/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2011 RELIC Authors
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
 * @defgroup pp Pairings over prime elliptic curves.
 */

/**
 * @file
 *
 * Interface of the bilinear pairing over elliptic curves functions.
 *
 * @version $Id$
 * @ingroup pp
 */

#ifndef RELIC_PP_H
#define RELIC_PP_H

#include "relic_fp.h"
#include "relic_ep.h"
#include "relic_types.h"

/*============================================================================*/
/* Type definitions                                                           */
/*============================================================================*/

/**
 * Represents a quadratic extension prime field element.
 *
 * This extension is constructed with the basis {1, B}, where B is an
 * adjoined square root in the prime field. If p = 3 mod 8, u = i, i^2 = -1;
 * otherwise u^2 = -2.
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

/**
 * Represents an elliptic curve point over a quadratic extension over a prime
 * field.
 */
typedef struct {
	/** The first coordinate. */
	fp2_t x;
	/** The second coordinate. */
	fp2_t y;
	/** The third coordinate (projective representation). */
	fp2_t z;
	/** Flag to indicate that this point is normalized. */
	int norm;
} ep2_st;

/**
 * Pointer to an elliptic curve point.
 */
#if ALLOC == AUTO
typedef ep2_st ep2_t[1];
#else
typedef ep2_st *ep2_t;
#endif

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
#if PP_QUD == BASIC
#define fp2_add(C, A, B)	fp2_add_basic(C, A, B)
#elif PP_QUD == INTEG
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
#if PP_QUD == BASIC
#define fp2_sub(C, A, B)	fp2_sub_basic(C, A, B)
#elif PP_QUD == INTEG
#define fp2_sub(C, A, B)	fp2_sub_integ(C, A, B)
#endif

/**
 * Doubles a quadratic extension field elements. Computes C = A + A.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the quadratic extension field element.
 */
#if PP_QUD == BASIC
#define fp2_dbl(C, A)		fp2_dbl_basic(C, A)
#elif PP_QUD == INTEG
#define fp2_dbl(C, A)		fp2_dbl_integ(C, A)
#endif

/**
 * Multiplies two quadratic extension field elements. Computes C = A * B.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the first quadratic extension field element.
 * @param[in] B				- the second quadratic extension field element.
 */
#if PP_QUD == BASIC
#define fp2_mul(C, A, B)	fp2_mul_basic(C, A, B)
#elif PP_QUD == INTEG
#define fp2_mul(C, A, B)	fp2_mul_integ(C, A, B)
#endif

/**
 * Squares a quadratic extension field elements. Computes C = A * A.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the quadratic extension field element to square.
 */
#if PP_QUD == BASIC
#define fp2_sqr(C, A)		fp2_sqr_basic(C, A)
#elif PP_QUD == INTEG
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

/**
 * Initializes a point on a elliptic curve with a null value.
 *
 * @param[out] A			- the point to initialize.
 */
#if ALLOC == AUTO
#define ep2_null(A)				/* empty */
#else
#define ep2_null(A)			A = NULL
#endif

/**
 * Calls a function to allocate a point on a elliptic curve.
 *
 * @param[out] A			- the new point.
 * @throw ERR_NO_MEMORY		- if there is no available memory.
 */
#if ALLOC == DYNAMIC
#define ep2_new(A)															\
	A = (ep2_t)calloc(1, sizeof(ep2_st));									\
	if (A == NULL) {														\
		THROW(ERR_NO_MEMORY);												\
	}																		\
	fp2_new((A)->x);														\
	fp2_new((A)->y);														\
	fp2_new((A)->z);														\

#elif ALLOC == STATIC
#define ep2_new(A)															\
	A = (ep2_t)alloca(sizeof(ep2_st));										\
	if (A == NULL) {														\
		THROW(ERR_NO_MEMORY);												\
	}																		\
	fp2_new((A)->x);														\
	fp2_new((A)->y);														\
	fp2_new((A)->z);														\

#elif ALLOC == AUTO
#define ep2_new(A)				/* empty */

#elif ALLOC == STACK
#define ep2_new(A)															\
	A = (ep2_t)alloca(sizeof(ep2_st));										\
	fp2_new((A)->x);														\
	fp2_new((A)->y);														\
	fp2_new((A)->z);														\

#endif

/**
 * Calls a function to clean and free a point on a elliptic curve.
 *
 * @param[out] A			- the point to free.
 */
#if ALLOC == DYNAMIC
#define ep2_free(A)															\
	if (A != NULL) {														\
		free(A);															\
		A = NULL;															\
	}

#elif ALLOC == STATIC
#define ep2_free(A)															\
	if (A != NULL) {														\
		fp2_free((A)->x);													\
		fp2_free((A)->y);													\
		fp2_free((A)->z);													\
		A = NULL;															\
	}																		\

#elif ALLOC == AUTO
#define ep2_free(A)				/* empty */

#elif ALLOC == STACK
#define ep2_free(A)															\
	A = NULL;																\

#endif

/**
 * Negates a point in an elliptic curve over a quadratic extension field.
 *
 * @param[out] R				- the result.
 * @param[in] P					- the point to negate.
 */
#if EP_ADD == BASIC
#define ep2_neg(R, P)			ep2_neg_basic(R, P)
#elif EP_ADD == PROJC
#define ep2_neg(R, P)			ep2_neg_projc(R, P)
#endif

/**
 * Adds two points in an elliptic curve over a quadratic extension field.
 * Computes R = P + Q.
 *
 * @param[out] R				- the result.
 * @param[in] P					- the first point to add.
 * @param[in] Q					- the second point to add.
 */
#if EP_ADD == BASIC
#define ep2_add(R, P, Q)		ep2_add_basic(R, P, Q);
#elif EP_ADD == PROJC
#define ep2_add(R, P, Q)		ep2_add_projc(R, P, Q);
#endif

/**
 * Adds two points in an elliptic curve over a quadratic extension field and
 * returns the computed slope. Computes R = P + Q and the slope S.
 *
 * @param[out] R				- the result.
 * @param[in] P					- the first point to add.
 * @param[in] Q					- the second point to add.
 */
#if EP_ADD == BASIC
#define ep2_add_slp(R, S, P, Q)	ep2_add_slp_basic(R, S, P, Q);
#elif EP_ADD == PROJC
#define ep2_add_slp(R, S, P, Q)	ep2_add_slp_projc(R, S, P, Q);
#endif

/**
 * Subtracts a point in an elliptic curve over a quadratic extension field from
 * another point in this curve. Computes R = P - Q.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the first point.
 * @param[in] Q				- the second point.
 */
#if EP_ADD == BASIC
#define ep2_sub(R, P, Q)		ep2_sub_basic(R, P, Q)
#elif EP_ADD == PROJC
#define ep2_sub(R, P, Q)		ep2_sub_projc(R, P, Q)
#endif

/**
 * Doubles a point in an elliptic curve over a quadratic extension field.
 * Computes R = 2 * P.
 *
 * @param[out] R				- the result.
 * @param[in] P					- the point to double.
 */
#if EP_ADD == BASIC
#define ep2_dbl(R, P)			ep2_dbl_basic(R, P);
#elif EP_ADD == PROJC
#define ep2_dbl(R, P)			ep2_dbl_projc(R, P);
#endif

/**
 * Doubles a point in an elliptic curve over a quadratic extension field and
 * returns the computed slope. Computes R = 2 * P and the slope S.
 *
 * @param[out] R				- the result.
 * @param[in] P					- the point to double.
 */
#if EP_ADD == BASIC
#define ep2_dbl_slp(R, S, E, P)	ep2_dbl_slp_basic(R, S, E, P);
#elif EP_ADD == PROJC
#define ep2_dbl_slp(R, S, E, P)	ep2_dbl_slp_projc(R, S, E, P);
#endif

/**
 * Builds a precomputation table for multiplying a fixed prime elliptic point
 * over a quadratic extension.
 *
 * @param[out] T			- the precomputation table.
 * @param[in] P				- the point to multiply.
 */
#if EP_FIX == BASIC
#define ep2_mul_pre(T, P)		ep2_mul_pre_basic(T, P)
#elif EP_FIX == YAOWI
#define ep2_mul_pre(T, P)		ep2_mul_pre_yaowi(T, P)
#elif EP_FIX == NAFWI
#define ep2_mul_pre(T, P)		ep2_mul_pre_nafwi(T, P)
#elif EP_FIX == COMBS
#define ep2_mul_pre(T, P)		ep2_mul_pre_combs(T, P)
#elif EP_FIX == COMBD
#define ep2_mul_pre(T, P)		ep2_mul_pre_combd(T, P)
#elif EP_FIX == LWNAF
#define ep2_mul_pre(T, P)		ep2_mul_pre_lwnaf(T, P)
#elif EP_FIX == GLV
//TODO: implement ep2_mul_pre_glv
#define ep2_mul_pre(T, P)		ep2_mul_pre_lwnaf(T, P)
#endif

/**
 * Multiplies a fixed prime elliptic point over a quadratic extension using a
 * precomputation table. Computes R = kP.
 *
 * @param[out] R			- the result.
 * @param[in] T				- the precomputation table.
 * @param[in] K				- the integer.
 */
#if EP_FIX == BASIC
#define ep2_mul_fix(R, T, K)		ep2_mul_fix_basic(R, T, K)
#elif EP_FIX == YAOWI
#define ep2_mul_fix(R, T, K)		ep2_mul_fix_yaowi(R, T, K)
#elif EP_FIX == NAFWI
#define ep2_mul_fix(R, T, K)		ep2_mul_fix_nafwi(R, T, K)
#elif EP_FIX == COMBS
#define ep2_mul_fix(R, T, K)		ep2_mul_fix_combs(R, T, K)
#elif EP_FIX == COMBD
#define ep2_mul_fix(R, T, K)		ep2_mul_fix_combd(R, T, K)
#elif EP_FIX == LWNAF
#define ep2_mul_fix(R, T, K)		ep2_mul_fix_lwnaf(R, T, K)
#elif EP_FIX == GLV
//TODO: implement ep2_mul_pre_glv
#define ep2_mul_fix(R, T, K)		ep2_mul_fix_lwnaf(R, T, K)
#endif

/**
 * Multiplies and adds two prime elliptic curve points simultaneously. Computes
 * R = kP + lQ.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the first point to multiply.
 * @param[in] K				- the first integer.
 * @param[in] Q				- the second point to multiply.
 * @param[in] L				- the second integer,
 */
#if EP_SIM == BASIC
#define ep2_mul_sim(R, P, K, Q, L)	ep2_mul_sim_basic(R, P, K, Q, L)
#elif EP_SIM == TRICK
#define ep2_mul_sim(R, P, K, Q, L)	ep2_mul_sim_trick(R, P, K, Q, L)
#elif EP_SIM == INTER
#define ep2_mul_sim(R, P, K, Q, L)	ep2_mul_sim_inter(R, P, K, Q, L)
#elif EP_SIM == JOINT
#define ep2_mul_sim(R, P, K, Q, L)	ep2_mul_sim_joint(R, P, K, Q, L)
#endif

/**
 * Computes the pairing of two binary elliptic curve points. Computes e(P, Q).
 *
 * @param[out] R			- the result.
 * @param[in] P				- the first elliptic curve point.
 * @param[in] Q				- the second elliptic curve point.
 */
#if PP_MAP == R_ATE
#define pp_map(R, P, Q)		pp_map_r_ate(R, P, Q)
#elif PP_MAP == O_ATE
#define pp_map(R, P, Q)		pp_map_o_ate(R, P, Q)
#elif PP_MAP == X_ATE
#define pp_map(R, P, Q)		pp_map_x_ate(R, P, Q)
#endif

/*============================================================================*/
/* Function prototypes                                                        */
/*============================================================================*/

/**
 * Initializes the pairing over elliptic curves.
 */
void pp_map_init(void);

/**
 * Finalizes the pairing over elliptic curves.
 */
void pp_map_clean(void);

/**
 * Initializes the constant needed to compute the Frobenius map.
 */
void fp2_const_init(void);

/**
 * Finalizes the constant needed to compute the Frobenius map.
 */
void fp2_const_clean(void);

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
 * Negates a quadratic extension field element.
 *
 * @param[out] C			- the result.
 * @param[out] A			- the quadratic extension field element to negate.
 */
void fp2_neg(fp2_t c, fp2_t a);

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
 * non-residue. Computes c = a * E, where E is a non-square/non-cube in the
 * quadratic extension.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension field element to multiply.
 */
void fp2_mul_nor(fp2_t c, fp2_t a);

/**
 * Multiplies a quadratic extension field element by a power of the constant
 * needed to compute the Frobenius map.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the field element to multiply.
 * @param[in] i				- the power of the constant.
 */
void fp2_mul_frb(fp2_t c, fp2_t a, int i);

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
 * Computes the Frobenius of a quadratic extension field element. This is the
 * same as computing the conjugate of the extension field element.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension element to conjugate.
 */
void fp2_frb(fp2_t c, fp2_t a);

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
 * Negates a sextic extension field element.
 *
 * @param[out] C			- the result.
 * @param[out] A			- the sextic extension field element to negate.
 */
void fp6_neg(fp6_t c, fp6_t a);

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
 * Computes the Frobenius endomorphism of a sextic extension field element.
 * Computes c = a^p.
 *
 * @param[out] c			- the result.
 * @param[in] a				- a sextic extension field element.
 * @param[in] b				- the constant v^{p-1} used for the computation.
 */
void fp6_frb(fp6_t c, fp6_t a);

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
 * Negates a dodecic extension field element.
 *
 * @param[out] C			- the result.
 * @param[out] A			- the dodecic extension field element to negate.
 */
void fp12_neg(fp12_t c, fp12_t a);

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
 */
void fp12_frb(fp12_t c, fp12_t a);

/**
 * Computes two consecutive Frobenius endomorphisms of a dodecic extension
 * element. Computes c = a^(p^2).
 *
 * @param[out] c			- the result.
 * @param[in] a				- a dodecic extension field element.
 */
void fp12_frb_sqr(fp12_t c, fp12_t a);

/**
 * Computes a power of a dodecic extension field element.
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
 * Initializes the elliptic curve over quadratic extension.
 */
void ep2_curve_init(void);

/**
 * Finalizes the elliptic curve over quadratic extension.
 */
void ep2_curve_clean(void);

/**
 * Returns the a coefficient of the currently configured elliptic curve.
 *
 * @param[out] a			- the a coefficient of the elliptic curve.
 */
void ep2_curve_get_a(fp2_t a);

/**
 * Returns the b coefficient of the currently configured elliptic curve.
 *
 * @param[out] b			- the b coefficient of the elliptic curve.
 */
void ep2_curve_get_b(fp2_t b);

/**
 * Returns a optimization identifier based on the coefficient a of the curve.
 *
 * @return the optimization identifier.
 */
int ep2_curve_opt_a(void);

/**
 * Tests if the configured elliptic curve is a twist.
 *
 * @return 1 if the elliptic curve is a twist, 0 otherwise.
 */
int ep2_curve_is_twist(void);

/**
 * Returns the generator of the group of points in the elliptic curve.
 *
 * @param[out] g			- the returned generator.
 */
void ep2_curve_get_gen(ep2_t g);

/**
 * Returns the precomputation table for the generator.
 *
 * @return the table.
 */
ep2_t *ep2_curve_get_tab(void);

/**
 * Returns the order of the group of points in the elliptic curve.
 *
 * @param[out] n			- the returned order.
 */
void ep2_curve_get_ord(bn_t n);

/**
 * Configures a new elliptic curve by using the curve over the base prime field
 * as a parameter.
 *
 *  @param				- the flag indicating that the curve is a twist.
 */
void ep2_curve_set(int twist);

/**
 * Tests if a point on a elliptic curve is at the infinity.
 *
 * @param[in] p				- the point to test.
 * @return 1 if the point is at infinity, 0 otherise.
 */
int ep2_is_infty(ep2_t p);

/**
 * Assigns a elliptic curve point to a point at the infinity.
 *
 * @param[out] p			- the point to assign.
 */
void ep2_set_infty(ep2_t p);

/**
 * Copies the second argument to the first argument.
 *
 * @param[out] q			- the result.
 * @param[in] p				- the elliptic curve point to copy.
 */
void ep2_copy(ep2_t r, ep2_t p);

/**
 * Compares two elliptic curve points.
 *
 * @param[in] p				- the first elliptic curve point.
 * @param[in] q				- the second elliptic curve point.
 * @return CMP_EQ if p == q and CMP_NE if p != q.
 */
int ep2_cmp(ep2_t p, ep2_t q);

/**
 * Assigns a random value to an elliptic curve point.
 *
 * @param[out] p			- the elliptic curve point to assign.
 */
void ep2_rand(ep2_t p);

/**
 * Computes the right-hand side of the elliptic curve equation at a certain
 * elliptic curve point.
 *
 * @param[out] rhs			- the result.
 * @param[in] p				- the point.
 */
void ep2_rhs(fp2_t rhs, ep2_t p);

/**
 * Tests if a point is in the curve.
 *
 * @param[in] p				- the point to test.
 */
int ep2_is_valid(ep2_t p);

/**
 * Builds a precomputation table for multiplying a random prime elliptic point.
 *
 * @param[out] t			- the precomputation table.
 * @param[in] p				- the point to multiply.
 * @param[in] w				- the window width.
 */
void ep2_tab(ep2_t * t, ep2_t p, int w);

/**
 * Prints a elliptic curve point.
 *
 * @param[in] p				- the elliptic curve point to print.
 */
void ep2_print(ep2_t p);

/**
 * Negates a point represented in affine coordinates in an elliptic curve over
 * a quadratic twist.
 *
 * @param[out] r			- the result.
 * @param[out] p			- the point to negate.
 */
void ep2_neg_basic(ep2_t r, ep2_t p);

/**
 * Negates a point represented in projective coordinates in an elliptic curve over
 * a quadratic twist.
 *
 * @param[out] r			- the result.
 * @param[out] p			- the point to negate.
 */
void ep2_neg_projc(ep2_t r, ep2_t p);

/**
 * Adds to points represented in affine coordinates in an elliptic curve over a
 * quadratic extension.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first point to add.
 * @param[in] q				- the second point to add.
 */
void ep2_add_basic(ep2_t r, ep2_t p, ep2_t q);

/**
 * Adds to points represented in affine coordinates in an elliptic curve over a
 * quadratic extension and returns the computed slope.
 *
 * @param[out] r			- the result.
 * @param[out] s			- the slope.
 * @param[in] p				- the first point to add.
 * @param[in] q				- the second point to add.
 */
void ep2_add_slp_basic(ep2_t r, fp2_t s, ep2_t p, ep2_t q);

/**
 * Subtracts a points represented in affine coordinates in an elliptic curve
 * over a quadratic extension from another point.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first point.
 * @param[in] q				- the point to subtract.
 */
void ep2_sub_basic(ep2_t r, ep2_t p, ep2_t q);

/**
 * Adds two points represented in projective coordinates in an elliptic curve
 * over a quadratic extension.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first point to add.
 * @param[in] q				- the second point to add.
 */
void ep2_add_projc(ep2_t r, ep2_t p, ep2_t q);

/**
 * Adds two points represented in projective coordinates in an elliptic curve
 * over a quadratic extension and returns the computed slope.
 *
 * @param[out] r			- the result.
 * @param[out] s			- the slope.
 * @param[in] p				- the first point to add.
 * @param[in] q				- the second point to add.
 */
void ep2_add_slp_projc(ep2_t r, fp2_t s, ep2_t p, ep2_t q);

/**
 * Subtracts a points represented in projective coordinates in an elliptic curve
 * over a quadratic extension from another point.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first point.
 * @param[in] q				- the point to subtract.
 */
void ep2_sub_projc(ep2_t r, ep2_t p, ep2_t q);

/**
 * Doubles a points represented in affine coordinates in an elliptic curve over
 * a quadratic extension.
 *
 * @param[out] r			- the result.
 * @param[int] p			- the point to double.
 */
void ep2_dbl_basic(ep2_t r, ep2_t p);

/**
 * Doubles a points represented in affine coordinates in an elliptic curve over
 * a quadratic extension and returns the computed slope.
 *
 * @param[out] r			- the result.
 * @param[out] s			- the numerator of the slope.
 * @param[out] e			- the denominator of the slope.
 * @param[in] p				- the point to double.
 */
void ep2_dbl_slp_basic(ep2_t r, fp2_t s, fp2_t e, ep2_t p);

/**
 * Doubles a points represented in projective coordinates in an elliptic curve
 * over a quadratic extension.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the point to double.
 */
void ep2_dbl_projc(ep2_t r, ep2_t p);

/**
 * Doubles a points represented in projective coordinates in an elliptic curve
 * over a quadratic extension and returns the computed slope.
 *
 * @param[out] r			- the result.
 * @param[out] s			- the numerator of the slope.
 * @param[out] e			- the denominator of the slope.
 * @param[in] p				- the point to double.
 */
void ep2_dbl_slp_projc(ep2_t r, fp2_t s, fp2_t e, ep2_t p);

/**
 * Multiplies a point in a elliptic curve over a quadratic extension by an
 * integer scalar.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the point to multiply.
 * @param[in] k				- the scalar.
 */
void ep2_mul(ep2_t r, ep2_t p, bn_t k);

/**
 * Multiplies the generator of an elliptic curve over a qaudratic extension.
 *
 * @param[out] r			- the result.
 * @param[in] k				- the integer.
 */
void ep2_mul_gen(ep2_t r, bn_t k);

/**
 * Builds a precomputation table for multiplying a fixed prime elliptic point
 * using the binary method.
 *
 * @param[out] t			- the precomputation table.
 * @param[in] p				- the point to multiply.
 */
void ep2_mul_pre_basic(ep2_t * t, ep2_t p);

/**
 * Builds a precomputation table for multiplying a fixed prime elliptic point
 * using Yao's windowing method.
 *
 * @param[out] t			- the precomputation table.
 * @param[in] p				- the point to multiply.
 */
void ep2_mul_pre_yaowi(ep2_t * t, ep2_t p);

/**
 * Builds a precomputation table for multiplying a fixed prime elliptic point
 * using the NAF windowing method.
 *
 * @param[out] t			- the precomputation table.
 * @param[in] p				- the point to multiply.
 */
void ep2_mul_pre_nafwi(ep2_t * t, ep2_t p);

/**
 * Builds a precomputation table for multiplying a fixed prime elliptic point
 * using the single-table comb method.
 *
 * @param[out] t			- the precomputation table.
 * @param[in] p				- the point to multiply.
 */
void ep2_mul_pre_combs(ep2_t * t, ep2_t p);

/**
 * Builds a precomputation table for multiplying a fixed prime elliptic point
 * using the double-table comb method.
 *
 * @param[out] t			- the precomputation table.
 * @param[in] p				- the point to multiply.
 */
void ep2_mul_pre_combd(ep2_t * t, ep2_t p);

/**
 * Builds a precomputation table for multiplying a fixed prime elliptic point
 * using the w-(T)NAF method.
 *
 * @param[out] t			- the precomputation table.
 * @param[in] p				- the point to multiply.
 */
void ep2_mul_pre_lwnaf(ep2_t * t, ep2_t p);

/**
 * Multiplies a fixed prime elliptic point using a precomputation table and
 * the binary method.
 *
 * @param[out] r			- the result.
 * @param[in] t				- the precomputation table.
 * @param[in] k				- the integer.
 */
void ep2_mul_fix_basic(ep2_t r, ep2_t * t, bn_t k);

/**
 * Multiplies a fixed prime elliptic point using a precomputation table and
 * Yao's windowing method
 *
 * @param[out] r			- the result.
 * @param[in] t				- the precomputation table.
 * @param[in] k				- the integer.
 */
void ep2_mul_fix_yaowi(ep2_t r, ep2_t * t, bn_t k);

/**
 * Multiplies a fixed prime elliptic point using a precomputation table and
 * the w-(T)NAF method.
 *
 * @param[out] r			- the result.
 * @param[in] t				- the precomputation table.
 * @param[in] k				- the integer.
 */
void ep2_mul_fix_nafwi(ep2_t r, ep2_t * t, bn_t k);

/**
 * Multiplies a fixed prime elliptic point using a precomputation table and
 * the single-table comb method.
 *
 * @param[out] r			- the result.
 * @param[in] t				- the precomputation table.
 * @param[in] k				- the integer.
 */
void ep2_mul_fix_combs(ep2_t r, ep2_t * t, bn_t k);

/**
 * Multiplies a fixed prime elliptic point using a precomputation table and
 * the double-table comb method.
 *
 * @param[out] r			- the result.
 * @param[in] t				- the precomputation table.
 * @param[in] k				- the integer.
 */
void ep2_mul_fix_combd(ep2_t r, ep2_t * t, bn_t k);

/**
 * Multiplies a fixed prime elliptic point using a precomputation table and
 * the w-(T)NAF method.
 *
 * @param[out] r			- the result.
 * @param[in] t				- the precomputation table.
 * @param[in] k				- the integer.
 */
void ep2_mul_fix_lwnaf(ep2_t r, ep2_t * t, bn_t k);

/**
 * Multiplies and adds two prime elliptic curve points simultaneously using
 * scalar multiplication and point addition.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first point to multiply.
 * @param[in] k				- the first integer.
 * @param[in] q				- the second point to multiply.
 * @param[in] l				- the second integer,
 */
void ep2_mul_sim_basic(ep2_t r, ep2_t p, bn_t k, ep2_t q, bn_t l);

/**
 * Multiplies and adds two prime elliptic curve points simultaneously using
 * shamir's trick.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first point to multiply.
 * @param[in] k				- the first integer.
 * @param[in] q				- the second point to multiply.
 * @param[in] l				- the second integer,
 */
void ep2_mul_sim_trick(ep2_t r, ep2_t p, bn_t k, ep2_t q, bn_t l);

/**
 * Multiplies and adds two prime elliptic curve points simultaneously using
 * interleaving of NAFs.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first point to multiply.
 * @param[in] k				- the first integer.
 * @param[in] q				- the second point to multiply.
 * @param[in] l				- the second integer,
 */
void ep2_mul_sim_inter(ep2_t r, ep2_t p, bn_t k, ep2_t q, bn_t l);

/**
 * Multiplies and adds two prime elliptic curve points simultaneously using
 * Solinas' Joint Sparse Form.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first point to multiply.
 * @param[in] k				- the first integer.
 * @param[in] q				- the second point to multiply.
 * @param[in] l				- the second integer,
 */
void ep2_mul_sim_joint(ep2_t r, ep2_t p, bn_t k, ep2_t q, bn_t l);

/**
 * Multiplies and adds the generator and a prime elliptic curve point
 * simultaneously. Computes R = kG + lQ.
 *
 * @param[out] r			- the result.
 * @param[in] k				- the first integer.
 * @param[in] q				- the second point to multiply.
 * @param[in] l				- the second integer,
 */
void ep2_mul_sim_gen(ep2_t r, bn_t k, ep2_t q, bn_t l);

/**
 * Multiplies a prime elliptic point by a small integer.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the point to multiply.
 * @param[in] k				- the integer.
 */
void ep2_mul_dig(ep2_t r, ep2_t p, dig_t k);

/**
 * Converts a point to affine coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the point to convert.
 */
void ep2_norm(ep2_t r, ep2_t p);

/**
 * Maps a byte array to a point in an elliptic curve over a quadratic extension.
 *
 * @param[out] p			- the result.
 * @param[in] msg			- the byte array to map.
 * @param[in] len			- the array length in bytes.
 */
void ep2_map(ep2_t p, unsigned char *msg, int len);

/**
 * Computes the Frobenius map of a point represented in affine coordinates
 * in an elliptic curve over a quadratic exension. Computes
 * Frob(P = (x, y)) = (x^p, y^p). On a quadratic twist, this is the same as
 * multiplying by the field characteristic.
 *
 * @param[out] r			- the result in affine coordinates.
 * @param[in] p				- a point in affine coordinates.
 */
void ep2_frb(ep2_t r, ep2_t p);

/**
 * Computes two consectutive Frobenius maps of a point represented in affine
 * coordinates in an elliptic curve over a quadratic exension. Computes
 * Frob(P = (x, y)) = (x^(p^2), y^(p^2)).
 *
 * @param[out] r			- the result in affine coordinates.
 * @param[in] p				- a point in affine coordinates.
 */
void ep2_frb_sqr(ep2_t r, ep2_t p);

/**
 * Initializes the pairing over prime fields.
 */
void pp_map_init(void);

/**
 * Finalizes the pairing over prime fields.
 */
void pp_map_clean(void);

/**
 * Computes the R-ate pairing of two points in a parameterized elliptic curve.
 * Computes e(Q, P).
 *
 * @param[out] r			- the result.
 * @param[in] q				- the first elliptic curve point.
 * @param[in] p				- the second elliptic curve point.
 */
void pp_map_r_ate(fp12_t r, ep_t p, ep2_t q);

/**
 * Computes the Optimal ate pairing of two points in a parameterized elliptic
 * curve. Computes e(Q, P).
 *
 * @param[out] r			- the result.
 * @param[in] q				- the first elliptic curve point.
 * @param[in] p				- the second elliptic curve point.
 */
void pp_map_o_ate(fp12_t r, ep_t p, ep2_t q);

/**
 * Computes the X-ate pairing of two points in a parameterized elliptic curve.
 * Computes e(Q, P).
 *
 * @param[out] r			- the result.
 * @param[in] q				- the first elliptic curve point.
 * @param[in] p				- the second elliptic curve point.
 */
void pp_map_x_ate(fp12_t r, ep_t p, ep2_t q);

#endif /* !RELIC_PP_H */
