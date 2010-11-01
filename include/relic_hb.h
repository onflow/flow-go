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
 * @defgroup hb hyperelliptic genus 2 curves over binary fields.
 */

/**
 * @file
 *
 * Interface of the binary hyperelliptic curves functions.
 *
 * @version $Id: relic_hb.h 390 2010-06-05 22:15:02Z dfaranha $
 * @ingroup hb
 */

#ifndef RELIC_HB_H
#define RELIC_HB_H

#include "relic_fb.h"
#include "relic_bn.h"
#include "relic_conf.h"
#include "relic_types.h"

/*============================================================================*/
/* Constant definitions                                                       */
/*============================================================================*/

/**
 * Binary hyperelliptic curve identifiers.
 */
enum {
	/** Pairing-friendly 113-bit hyperelliptic curve. */
	ETAT_S113 = 1,
	/** Pairing-friendly 367-bit hyperelliptic curve. */
	ETAT_S367 = 2,
	/** Pairing-friendly 439-bit hyperelliptic curve. */
	ETAT_S439 = 3,
};

/**
 * Size of a precomputation table using the binary method.
 */
#define HB_TABLE_BASIC		(2 * FB_BITS)

/**
 * Size of a precomputation table using Yao's windowing method.
 */
#define HB_TABLE_YAOWI      ((2 * FB_BITS) / HB_DEPTH + 1)

/**
 * Size of a precomputation table using the NAF windowing method.
 */
#define HB_TABLE_NAFWI      ((2 * FB_BITS) / HB_DEPTH + 1)

/**
 * Size of a precomputation table using the single-table comb method.
 */
#define HB_TABLE_COMBS      (1 << HB_DEPTH)

/**
 * Size of a precomputation table using the double-table comb method.
 */
#define HB_TABLE_COMBD		(1 << (HB_DEPTH + 1))

/**
 * Size of a precomputation table using the w-(T)NAF method.
 */
#define HB_TABLE_WTNAF		(1 << (HB_DEPTH - 2))

/**
 * Size of a precomputation table using the w-(T)NAF method.
 */
#define HB_TABLE_OCTUP		(8)

/**
 * Size of a precomputation table using the chosen algorithm.
 */
#if HB_FIX == BASIC
#define HB_TABLE			HB_TABLE_BASIC
#elif HB_FIX == YAOWI
#define HB_TABLE			HB_TABLE_YAOWI
#elif HB_FIX == NAFWI
#define HB_TABLE			HB_TABLE_NAFWI
#elif HB_FIX == COMBS
#define HB_TABLE			HB_TABLE_COMBS
#elif HB_FIX == COMBD
#define HB_TABLE			HB_TABLE_COMBD
#elif HB_FIX == WTNAF
#define HB_TABLE			HB_TABLE_WTNAF
#elif HB_FIX == OCTUP
#define HB_TABLE			HB_TABLE_OCTUP
#endif

/**
 * Maximum size of a precomputation table.
 */
#ifdef STRIP
#define HB_TABLE_MAX HB_TABLE
#else
#define HB_TABLE_MAX HB_TABLE_BASIC
#endif

/*============================================================================*/
/* Type definitions                                                           */
/*============================================================================*/

/**
 * Represents a hyperelliptic divisor class using the Mumford representation
 * [u,v] = [x^2 + u1 * x + u0, v1 * x + v0] over a binary field.
 */
typedef struct {
#if ALLOC == STATIC
	/** The x coefficient of polynomial u. */
	fb_t u1;
	/** The free term of polynomial u. */
	fb_t u0;
	/** The x coefficient of polynomial v. */
	fb_t v1;
	/** The free term of polynomial v. */
	fb_t v0;
#elif ALLOC == DYNAMIC || ALLOC == STACK || ALLOC == AUTO
	/** The x coefficient of polynomial u. */
	fb_st u1;
	/** The free term of polynomial u. */
	fb_st u0;
	/** The x coefficient of polynomial v. */
	fb_st v1;
	/** The free term of polynomial v. */
	fb_st v0;
#endif
	/** Flag to mark if this divisor is degenerate or not. */
	int deg;
} hb_st;

/**
 * Pointer to a hyperelliptic curve divisor class.
 */
#if ALLOC == AUTO
typedef hb_st hb_t[1];
#else
typedef hb_st *hb_t;
#endif

/*============================================================================*/
/* Macro definitions                                                          */
/*============================================================================*/

/**
 * Initializes a divisor class on a binary hyperelliptic curve with a null
 * value.
 *
 * @param[out] A			- the divisor class to initialize.
 */
#if ALLOC == AUTO
#define hb_null(A)		/* empty */
#else
#define hb_null(A)		A = NULL;
#endif

/**
 * Calls a function to allocate a divisor class on a binary hyperelliptic curve.
 *
 * @param[out] A			- the new divisor class.
 * @throw ERR_NO_MEMORY		- if there is no available memory.
 */
#if ALLOC == DYNAMIC
#define hb_new(A)															\
	A = (hb_t)calloc(1, sizeof(hb_st));										\
	if (A == NULL) {														\
		THROW(ERR_NO_MEMORY);												\
	}																		\

#elif ALLOC == STATIC
#define hb_new(A)															\
	A = (hb_t)alloca(sizeof(hb_st));										\
	if (A == NULL) {														\
		THROW(ERR_NO_MEMORY);												\
	}																		\
	fb_new((A)->u0);														\
	fb_new((A)->u1);														\
	fb_new((A)->v0);														\
	fb_new((A)->v1);														\

#elif ALLOC == AUTO
#define hb_new(A)			/* empty */

#elif ALLOC == STACK
#define hb_new(A)															\
	A = (hb_t)alloca(sizeof(hb_st));										\

#endif

/**
 * Calls a function to clean and free a divisor class on a binary hyperelliptic
 * curve.
 *
 * @param[out] A			- the divisor class to clean and free.
 */
#if ALLOC == DYNAMIC
#define hb_free(A)															\
	if (A != NULL) {														\
		free(A);															\
		A = NULL;															\
	}																		\

#elif ALLOC == STATIC
#define hb_free(A)															\
	if (A != NULL) {														\
		fb_free((A)->u0);													\
		fb_free((A)->u1);													\
		fb_free((A)->v0);													\
		fb_free((A)->v1);													\
		A = NULL;															\
	}																		\

#elif ALLOC == AUTO
#define hb_free(A)			/* empty */

#elif ALLOC == STACK
#define hb_free(A)															\
	A = NULL;																\

#endif

/**
 * Negates a binary hyperelliptic curve divisor class. Computes R = -P.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the divisor class to negate.
 */
//#if HB_ADD == BASIC
#define hb_neg(R, P)		hb_neg_basic(R, P)
//#endif

/**
 * Adds two binary hyperelliptic curve divisor classes. Computes R = P + Q.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the first divisor class to add.
 * @param[in] Q				- the second divisor class to add.
 */
//#if HB_ADD == BASIC
#define hb_add(R, P, Q)		hb_add_basic(R, P, Q);
//#endif

/**
 * Subtracts a binary hyperelliptic curve divisor class from another.
 * Computes R = P - Q.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the first divisor class.
 * @param[in] Q				- the second divisor class.
 */
//#if HB_ADD == BASIC
#define hb_sub(R, P, Q)		hb_sub_basic(R, P, Q)
//#endif

/**
 * Doubles a binary hyperelliptic curve divisor class. Computes R = P + P.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the divisor class to double.
 */
//#if HB_ADD == BASIC
#define hb_dbl(R, P)		hb_dbl_basic(R, P);
//#endif

/**
 * Octuples a binary hyperelliptic curve divisor class. Computes R = P + P.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the divisor class to octuple.
 */
//#if HB_ADD == BASIC
#define hb_oct(R, P)		hb_oct_basic(R, P);
//#endif

/**
 * Multiplies a binary hyperelliptic curve divisor class by an integer. Computes
 * R = kP.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the divisor class to multiply.
 * @param[in] K				- the integer.
 */
#if HB_MUL == BASIC
#define hb_mul(R, P, K)		hb_mul_basic(R, P, K)
#elif HB_MUL == OCTUP
#define hb_mul(R, P, K)		hb_mul_octup(R, P, K)
#elif HB_MUL == WTNAF
#define hb_mul(R, P, K)		hb_mul_wtnaf(R, P, K)
#endif

/**
 * Builds a precomputation table for multiplying a fixed binary hyperelliptic
 * curve point.
 *
 * @param[out] T			- the precomputation table.
 * @param[in] P				- the point to multiply.
 */
#if HB_FIX == BASIC
#define hb_mul_pre(T, P)		hb_mul_pre_basic(T, P)
#elif HB_FIX == YAOWI
#define hb_mul_pre(T, P)		hb_mul_pre_yaowi(T, P)
#elif HB_FIX == NAFWI
#define hb_mul_pre(T, P)		hb_mul_pre_nafwi(T, P)
#elif HB_FIX == COMBS
#define hb_mul_pre(T, P)		hb_mul_pre_combs(T, P)
#elif HB_FIX == COMBD
#define hb_mul_pre(T, P)		hb_mul_pre_combd(T, P)
#elif HB_FIX == WTNAF
#define hb_mul_pre(T, P)		hb_mul_pre_wtnaf(T, P)
#elif HB_FIX == OCTUP
#define hb_mul_pre(T, P)		hb_mul_pre_octup(T, P)
#endif

/**
 * Multiplies a fixed binary hyperelliptic point using a precomputation table.
 * Computes R = kP.
 *
 * @param[out] R			- the result.
 * @param[in] T				- the precomputation table.
 * @param[in] K				- the integer.
 */
#if HB_FIX == BASIC
#define hb_mul_fix(R, T, K)		hb_mul_fix_basic(R, T, K)
#elif HB_FIX == YAOWI
#define hb_mul_fix(R, T, K)		hb_mul_fix_yaowi(R, T, K)
#elif HB_FIX == NAFWI
#define hb_mul_fix(R, T, K)		hb_mul_fix_nafwi(R, T, K)
#elif HB_FIX == COMBS
#define hb_mul_fix(R, T, K)		hb_mul_fix_combs(R, T, K)
#elif HB_FIX == COMBD
#define hb_mul_fix(R, T, K)		hb_mul_fix_combd(R, T, K)
#elif HB_FIX == WTNAF
#define hb_mul_fix(R, T, K)		hb_mul_fix_wtnaf(R, T, K)
#elif HB_FIX == OCTUP
#define hb_mul_fix(R, T, K)		hb_mul_fix_octup(R, T, K)
#endif

/**
 * Multiplies and adds two binary hyperelliptic curve divisor classes
 * simultaneously. Computes R = kP + lQ.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the first point to multiply.
 * @param[in] K				- the first integer.
 * @param[in] Q				- the second point to multiply.
 * @param[in] L				- the second integer,
 */
#if HB_SIM == BASIC
#define hb_mul_sim(R, P, K, Q, L)	hb_mul_sim_basic(R, P, K, Q, L)
#elif HB_SIM == TRICK
#define hb_mul_sim(R, P, K, Q, L)	hb_mul_sim_trick(R, P, K, Q, L)
#elif HB_SIM == INTER
#define hb_mul_sim(R, P, K, Q, L)	hb_mul_sim_inter(R, P, K, Q, L)
#elif HB_SIM == JOINT
#define hb_mul_sim(R, P, K, Q, L)	hb_mul_sim_joint(R, P, K, Q, L)
#endif

/**
 * Renames hyperelliptic curve arithmetic operations to build precomputation
 * tables with the right coordinate system.
 */
#if defined(HB_MIXED)
/** @{ */
#define hb_add_tab			hb_add_basic
#define hb_sub_tab			hb_sub_basic
#define hb_neg_tab			hb_neg_basic
#define hb_dbl_tab			hb_dbl_basic
#define hb_frb_tab			hb_frb_basic
/** @} */
#else
/**@{ */
#define hb_add_tab			hb_add
#define hb_sub_tab			hb_sub
#define hb_neg_tab			hb_neg
#define hb_dbl_tab			hb_dbl
#define hb_frb_tab			hb_frb
/** @} */
#endif

/*============================================================================*/
/* Function prototypes                                                        */
/*============================================================================*/

/**
 * Initializes the binary hyperelliptic curve arithmetic module.
 */
void hb_curve_init(void);

/**
 * Finalizes the binary hyperelliptic curve arithmetic module.
 */
void hb_curve_clean(void);

/**
 * Returns the f0 coefficient of the currently configured binary hyperelliptic
 * curve.
 *
 * @return the f0 coefficient of the hyperelliptic curve.
 */
dig_t *hb_curve_get_f0(void);

/**
 * Returns the f1 coefficient of the currently configured supersingular binary
 * hyperelliptic curve.
 *
 * @return the f1 coefficient of the supersingular hyperelliptic curve.
 */
dig_t *hb_curve_get_f1(void);

/**
 * Returns the f3 coefficient of the currently configured binary hyperelliptic
 * curve.
 *
 * @return the f3 coefficient of the hyperelliptic curve.
 */
dig_t *hb_curve_get_f3(void);

/**
 * Returns a optimization identifier based on the coefficient f0 of the curve.
 *
 * @return the optimization identifier.
 */
int hb_curve_opt_f0(void);

/**
 * Returns a optimization identifier based on the coefficient f1 of the curve.
 *
 * @return the optimization identifier.
 */
int hb_curve_opt_f1(void);

/**
 * Returns a optimization identifier based on the coefficient f3 of the curve.
 *
 * @return the optimization identifier.
 */
int hb_curve_opt_f3(void);

/**
 * Tests if the configured binary hyperelliptic curve is supersingular.
 *
 * @return 1 if the binary hyperelliptic curve is supersingular, 0 otherwise.
 */
int hb_curve_is_super(void);

/**
 * Returns the generator of the jacobian in the binary hyperelliptic curve.
 *
 * @param[out] g			- the returned generator.
 */
void hb_curve_get_gen(hb_t g);

/**
 * Returns the precomputation table for the generator.
 *
 * @return the table.
 */
hb_t *hb_curve_get_tab(void);

/**
 * Returns the order of the jacobian in the binary hyperelliptic curve.
 *
 * @param[out] n			- the returned order.
 */
void hb_curve_get_ord(bn_t n);

/**
 * Returns the cofactor of the binary hyperelliptic curve.
 *
 * @param[out] n			- the returned cofactor.
 */
void hb_curve_get_cof(bn_t h);

/**
 * Configures a new supersingular binary hyperelliptic curve by its coefficients
 * and generator.
 *
 * @param[in] f3			- the coefficient a of the curve.
 * @param[in] f1			- the coefficient b of the curve.
 * @param[in] f0			- the coefficient c of the curve.
 * @param[in] g				- the generator.
 * @param[in] n				- the order of the generator.
 * @param[in] h				- the cofactor.
 */
void hb_curve_set_super(fb_t f3, fb_t f1, fb_t f0, hb_t g, bn_t n, bn_t h);

/**
 * Returns the parameter identifier of the currently configured binary
 * hyperelliptic curve.
 *
 * @return the parameter identifier.
 */
int hb_param_get(void);

/**
 * Configures a new binary hyperelliptic curve by its parameter identifier.
 *
 * @param[in] param			- the parameters identifier.
 */
void hb_param_set(int param);

/**
 * Configures some set of curve parameters for the current security level.
 */
int hb_param_set_any(void);

/**
 * Configures some set of supersingular curve parameters for the current
 * security level.
 *
 * @return STS_OK if there is a curve at this security level, STS_ERR otherwise.
 */
int hb_param_set_any_super(void);

/**
 * Prints the current configured binary hyperelliptic curve.
 */
void hb_param_print(void);

/**
 * Tests if a divisor class on a binary hyperelliptic curve is at the infinity.
 *
 * @param[in] p				- the divisor class to test.
 * @return 1 if the divisor class is at infinity, 0 otherise.
 */
int hb_is_infty(hb_t p);

/**
 * Assigns a binary hyperelliptic curve divisor class to the infinity.
 *
 * @param[out] p			- the divisor class to assign.
 */
void hb_set_infty(hb_t p);

/**
 * Copies the second argument to the first argument.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the binary hyperelliptic divisor to copy.
 */
void hb_copy(hb_t r, hb_t p);

/**
 * Compares two binary hyperelliptic curve divisor classes.
 *
 * @param[in] p				- the first binary hyperelliptic divisor class.
 * @param[in] q				- the second binary hyperelliptic divisor class.
 * @return CMP_EQ if p == q and CMP_NE if p != q.
 */
int hb_cmp(hb_t p, hb_t q);

/**
 * Assigns a random value to a binary hyperelliptic curve divisor class. The
 * probability is 50% for obtaining a degenerate divisor, 25% for a Type-A non-
 * degenerate divisor and 25% for a Type-B non-degenerate divisor.
 *
 * @param[out] p			- the binary hyperelliptic curve divisor to assign.
 */
void hb_rand(hb_t p);

/**
 * Assigns a random degenerate divisor to a binary hyperelliptic curve divisor
 * class.
 *
 * @param[out] p			- the binary hyperelliptic curve divisor to assign.
 */
void hb_rand_deg(hb_t p);

/**
 * Assigns a random non-degenerate divisor to a binary hyperelliptic curve
 * divisor class. If type = 0, the points corresponding to the divisor have
 * coordinates in the base field; otherwise the coordinates are in a quadratic
 * extension and the points are q-conjugates of each other.
 *
 * @param[out] p			- the binary hyperelliptic curve divisor to assign.
 * @param[in]				- the type of the divisor.
 */
void hb_rand_non(hb_t p, int type);

/**
 * Prints a binary hyperelliptic curve divisor class.
 *
 * @param[in] p				- the binary hyperelliptic divisor class to print.
 */
void hb_print(hb_t p);

/**
 * Negates a binary hyperelliptic curve divisor class represented by affine
 * coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the divisor class to negate.
 */
void hb_neg_basic(hb_t r, hb_t p);

/**
 * Adds two binary hyperelliptic curve divisor classes represented in affine
 * coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first divisor class to add.
 * @param[in] q				- the second divisor class to add.
 */
void hb_add_basic(hb_t r, hb_t p, hb_t q);

/**
 * Subtracts a binary hyperelliptic curve divisor class from another, both
 * divisor classes represented by affine coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first divisor class.
 * @param[in] q				- the second divisor class.
 */
void hb_sub_basic(hb_t r, hb_t p, hb_t q);

/**
 * Doubles a binary hyperelliptic curve divisor class represented in affine
 * coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the divisor class to double.
 */
void hb_dbl_basic(hb_t r, hb_t p);

/**
 * OCtuples a binary hyperelliptic curve divisor class represented in affine
 * coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the divisor class to octuple.
 */
void hb_oct_basic(hb_t r, hb_t p);

/**
 * Multiplies a binary hyperelliptic divisor class by an integer using the
 * binary method.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the divisor class to multiply.
 * @param[in] k				- the integer.
 */
void hb_mul_basic(hb_t r, hb_t p, bn_t k);

/**
 * Multiplies a divisor class of a binary hyperelliptic curve by an integer
 * using the w-NAF method.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the divisor class to multiply.
 * @param[in] k				- the integer.
 */
void hb_mul_wtnaf(hb_t r, hb_t p, bn_t k);

/**
 * Multiplies a divisor class of a binary hyperelliptic curve by an integer
 * using the octupling-based method.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the divisor class to multiply.
 * @param[in] k				- the integer.
 */
void hb_mul_octup(hb_t r, hb_t p, bn_t k);

/**
 * Multiplies the generator of a binary hyperelliptic curve by an integer.
 *
 * @param[out] r			- the result.
 * @param[in] k				- the integer.
 */
void hb_mul_gen(hb_t r, bn_t k);

/**
 * Multiplies a binary hyperelliptic divisor class by a small integer.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the divisor class to multiply.
 * @param[in] k				- the integer.
 */
void hb_mul_dig(hb_t r, hb_t p, dig_t k);

/**
 * Builds a precomputation table for multiplying a fixed binary hyperelliptic
 * divisor classss using the binary method.
 *
 * @param[out] t			- the precomputation table.
 * @param[in] p				- the divisor class to multiply.
 */
void hb_mul_pre_basic(hb_t *t, hb_t p);

/**
 * Builds a precomputation table for multiplying a fixed binary hyperelliptic
 * curve divisor class using Yao's windowing method.
 *
 * @param[out] t			- the precomputation table.
 * @param[in] p				- the divisor class to multiply.
 */
void hb_mul_pre_yaowi(hb_t *t, hb_t p);

/**
 * Builds a precomputation table for multiplying a fixed binary hyperelliptic
 * curve divisor class using the NAF windowing method.
 *
 * @param[out] t			- the precomputation table.
 * @param[in] p				- the divisor class to multiply.
 */
void hb_mul_pre_nafwi(hb_t *t, hb_t p);

/**
 * Builds a precomputation table for multiplying a fixed binary hyperelliptic
 * curve using the single-table comb method.
 *
 * @param[out] t			- the precomputation table.
 * @param[in] p				- the divisor class to multiply.
 */
void hb_mul_pre_combs(hb_t *t, hb_t p);

/**
 * Builds a precomputation table for multiplying a fixed binary hyperelliptic
 * curve divisor class using the double-table comb method.
 *
 * @param[out] t			- the precomputation table.
 * @param[in] p				- the divisor class to multiply.
 */
void hb_mul_pre_combd(hb_t *t, hb_t p);

/**
 * Builds a precomputation table for multiplying a fixed binary hyperelliptic
 * curve divisor class using the w-(T)NAF method.
 *
 * @param[out] t			- the precomputation table.
 * @param[in] p				- the divisor class to multiply.
 */
void hb_mul_pre_wtnaf(hb_t *t, hb_t p);

/**
 * Builds a precomputation table for multiplying a fixed binary hyperelliptic
 * curve divisor class using the octupling method.
 *
 * @param[out] t			- the precomputation table.
 * @param[in] p				- the divisor class to multiply.
 */
void hb_mul_pre_octup(hb_t *t, hb_t p);

/**
 * Multiplies a fixed binary hyperelliptic divisor class using a precomputation
 * table and the binary method.
 *
 * @param[out] r			- the result.
 * @param[in] t				- the precomputation table.
 * @param[in] k				- the integer.
 */
void hb_mul_fix_basic(hb_t r, hb_t *t, bn_t k);

/**
 * Multiplies a fixed binary hyperelliptic divisor class using a precomputation
 * table and Yao's windowing method
 *
 * @param[out] r			- the result.
 * @param[in] t				- the precomputation table.
 * @param[in] k				- the integer.
 */
void hb_mul_fix_yaowi(hb_t r, hb_t *t, bn_t k);

/**
 * Multiplies a fixed binary hyperelliptic divisor class using a precomputation
 * table and the w-(T)NAF method.
 *
 * @param[out] r			- the result.
 * @param[in] t				- the precomputation table.
 * @param[in] k				- the integer.
 */
void hb_mul_fix_nafwi(hb_t r, hb_t *t, bn_t k);

/**
 * Multiplies a fixed binary hyperelliptic divisor class using a precomputation
 * table and the single-table comb method.
 *
 * @param[out] r			- the result.
 * @param[in] t				- the precomputation table.
 * @param[in] k				- the integer.
 */
void hb_mul_fix_combs(hb_t r, hb_t *t, bn_t k);

/**
 * Multiplies a fixed binary hyperelliptic divisor class using a precomputation
 * table and the double-table comb method.
 *
 * @param[out] r			- the result.
 * @param[in] t				- the precomputation table.
 * @param[in] k				- the integer.
 */
void hb_mul_fix_combd(hb_t r, hb_t *t, bn_t k);

/**
 * Multiplies a fixed binary hyperelliptic divisor class using a precomputation
 * table and the w-(T)NAF method.
 *
 * @param[out] r			- the result.
 * @param[in] t				- the precomputation table.
 * @param[in] k				- the integer.
 */
void hb_mul_fix_wtnaf(hb_t r, hb_t *t, bn_t k);

/**
 * Multiplies a fixed binary hyperelliptic divisor class using a precomputation
 * table and the octupling method.
 *
 * @param[out] r			- the result.
 * @param[in] t				- the precomputation table.
 * @param[in] k				- the integer.
 */
void hb_mul_fix_octup(hb_t r, hb_t *t, bn_t k);

/**
 * Multiplies and adds two binary hyperelliptic curve divisor classes
 * simultaneously using scalar multiplication and divisor class addition.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first divisor class to multiply.
 * @param[in] k				- the first integer.
 * @param[in] q				- the second divisor class to multiply.
 * @param[in] l				- the second integer,
 */
void hb_mul_sim_basic(hb_t r, hb_t p, bn_t k, hb_t q, bn_t l);

/**
 * Multiplies and adds two binary hyperelliptic curve divisor classes
 * simultaneously using Shamir's trick.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first divisor class to multiply.
 * @param[in] k				- the first integer.
 * @param[in] q				- the second divisor class to multiply.
 * @param[in] l				- the second integer,
 */
void hb_mul_sim_trick(hb_t r, hb_t p, bn_t k, hb_t q, bn_t l);

/**
 * Multiplies and adds two binary hyperelliptic curve divisor classes
 * simultaneously using interleaving of NAFs.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first divisor class to multiply.
 * @param[in] k				- the first integer.
 * @param[in] q				- the second divisor class to multiply.
 * @param[in] l				- the second integer,
 */
void hb_mul_sim_inter(hb_t r, hb_t p, bn_t k, hb_t q, bn_t l);

/**
 * Multiplies and adds two binary hyperelliptic curve divisor classes simultaneously using
 * Solinas' Joint Sparse Form.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first divisor class to multiply.
 * @param[in] k				- the first integer.
 * @param[in] q				- the second divisor class to multiply.
 * @param[in] l				- the second integer,
 */
void hb_mul_sim_joint(hb_t r, hb_t p, bn_t k, hb_t q, bn_t l);

/**
 * Multiplies and adds the generator and a divisor class simultaneously.
 * Computes R = kG + lQ.
 *
 * @param[out] r			- the result.
 * @param[in] k				- the first integer.
 * @param[in] q				- the second divisor class to multiply.
 * @param[in] l				- the second integer,
 */
void hb_mul_sim_gen(hb_t r, bn_t k, hb_t q, bn_t l);

/**
 * Converts a divisor class to affine coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the divisor class to convert.
 */
void hb_norm(hb_t r, hb_t p);

#endif /* !RELIC_HB_H */
