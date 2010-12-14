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
 * Implementation of configuration of prime elliptic curves over quadratic
 * extensions.
 *
 * @version $Id: relic_pp_ep2.c 463 2010-07-13 21:12:13Z conradoplg $
 * @ingroup pp
 */

#include "relic_core.h"
#include "relic_md.h"
#include "relic_pp.h"
#include "relic_error.h"
#include "relic_conf.h"
#include "relic_fp_low.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if defined(EP_ORDIN) && FP_PRIME == 158
/**
 * Parameters for a pairing-friendly prime curve over a quadratic extension.
 */
/** @{ */
#define BN_P158_A0		"0"
#define BN_P158_A1		"0"
#define BN_P158_B0		"1"
#define BN_P158_B1		"240000006ED000007FE9C000419FEC800CA035C6"
#define BN_P158_X0		"1A7C449590DCF3E9407FB8525FB45A147A77AC47"
#define BN_P158_X1		"057FE1D8E1401E3C848C5A2E86447DEE3FF59BD0"
#define BN_P158_Y0		"0206982EB9A0B5AC1CC84B010CA25E5DDFDE63B1"
#define BN_P158_Y1		"1AF3EB03CFF8840D4D2ACF8E30E1BB1364020547"
#define BN_P158_R		"240000006ED000007FE96000419F59800C9FFD81"
/** @} */
#endif

#if defined(EP_ORDIN) && FP_PRIME == 254
/**
 * Parameters for a pairing-friendly prime curve over a quadratic extension.
 */
/** @{ */
#define BN_P254_A0		"0"
#define BN_P254_A1		"0"
#define BN_P254_B0		"1"
#define BN_P254_B1		"2523648240000001BA344D80000000086121000000000013A700000000000012"
#define BN_P254_X0		"061A10BB519EB62FEB8D8C7E8C61EDB6A4648BBB4898BF0D91EE4224C803FB2B"
#define BN_P254_X1		"0516AAF9BA737833310AA78C5982AA5B1F4D746BAE3784B70D8C34C1E7D54CF3"
#define BN_P254_Y0		"021897A06BAF93439A90E096698C822329BD0AE6BDBE09BD19F0E07891CD2B9A"
#define BN_P254_Y1		"0EBB2B0E7C8B15268F6D4456F5F38D37B09006FFD739C9578A2D1AEC6B3ACE9B"
#define BN_P254_R		"2523648240000001BA344D8000000007FF9F800000000010A10000000000000D"
/** @} */
#endif

#if defined(EP_ORDIN) && FP_PRIME == 256
/**
 * Parameters for a pairing-friendly prime curve over a quadratic extension.
 */
/** @{ */
#define BN_P256_A0		"0"
#define BN_P256_A1		"0"
#define BN_P256_B0		"1"
#define BN_P256_B1		"B64000000000ECBF9E00000073543404300018F825373836C206F994412505BE"
#define BN_P256_X0		"4E9CC6BC6CD0B6F4A64189D362F3441A33F7ECEFA1BCBC8EE1962261F6799383"
#define BN_P256_X1		"8E0B49853E5DD8850F228D0D3338470DE76515A084014A4D159590F362D7F387"
#define BN_P256_Y0		"0939AC72CF41569A0E045143558A396B14559F5D0898E687D9CAC72191B4DFA8"
#define BN_P256_Y1		"3A112DC12DBD31123F36CBEDAADE233BE4CAD713F15D5B679BB11FAFCD49728A"
#define BN_P256_R		"B64000000000ECBF9E00000073543403580018F82536ABEC4206F9942A5D7249"
/** @} */
#endif

/**
 * The generator of the elliptic curve.
 */
static ep2_st curve_g;

/**
 * The first coordinate of the generator.
 */
static fp2_st curve_gx;

/**
 * The second coordinate of the generator.
 */
static fp2_st curve_gy;

/**
 * The third coordinate of the generator.
 */
static fp2_st curve_gz;

/**
 * The a parameter of the curve.
 */
static fp2_st curve_a;

/**
 * The b parameter of the curve.
 */
static fp2_st curve_b;

/**
 * The order of the group of points in the elliptic curve.
 */
static bn_st curve_r;

/**
 * Flag that stores if the configured prime elliptic curve is twisted.
 */
static int curve_is_twist;

#ifdef EP_PRECO

/**
 * Precomputation table for generator multiplication.
 */
static ep2_st table[EP_TABLE];

/**
 * Array of pointers to the precomputation table.
 */
static ep2_st *pointer[EP_TABLE];

#endif

/*============================================================================*/
	/* Public definitions                                                         */
/*============================================================================*/

void ep2_curve_init(void) {
#ifdef EP_PRECO
	for (int i = 0; i < EP_TABLE; i++) {
		pointer[i] = &(table[i]);
	}
#endif
#if ALLOC == STATIC
	fp2_new(curve_gx);
	fp2_new(curve_gy);
	fp2_new(curve_gz);
#endif
#if ALLOC == AUTO
	fp2_copy(curve_g.x, curve_gx);
	fp2_copy(curve_g.y, curve_gy);
	fp2_copy(curve_g.z, curve_gz);
#else
	curve_g.x[0] = curve_gx[0];
	curve_g.x[1] = curve_gx[1];
	curve_g.y[0] = curve_gy[0];
	curve_g.y[1] = curve_gy[1];
	curve_g.z[0] = curve_gz[0];
	curve_g.z[1] = curve_gz[1];
#endif
	ep2_set_infty(&curve_g);
	bn_init(&curve_r, FP_DIGS);
}

void ep2_curve_clean(void) {
#if ALLOC == STATIC
	fp2_free(curve_gx);
	fp2_free(curve_gy);
	fp2_free(curve_gz);
#endif
	bn_clean(&curve_r);
}

int ep2_curve_is_twist() {
	return curve_is_twist;
}

void ep2_curve_get_gen(ep2_t g) {
	ep2_copy(g, &curve_g);
}

void ep2_curve_get_a(fp2_t a) {
	fp_copy(a[0], curve_a[0]);
	fp_copy(a[1], curve_a[1]);
}

void ep2_curve_get_b(fp2_t b) {
	fp_copy(b[0], curve_b[0]);
	fp_copy(b[1], curve_b[1]);
}

void ep2_curve_get_ord(bn_t n) {
	if (curve_is_twist) {
		ep_curve_get_ord(n);
	} else {
		bn_copy(n, &curve_r);
	}
}

#if defined(EP_PRECO)

ep2_t *ep2_curve_get_tab() {
#if ALLOC == AUTO
	return (ep2_t*) *pointer;
#else
	return pointer;
#endif
}

#endif

void ep2_curve_set(int twist) {
	int param;
	char *str;
	ep2_t g;
	fp2_t a;
	fp2_t b;
	bn_t r;

	ep2_null(g);
	fp2_null(a);
	fp2_null(b);
	bn_null(r);

	TRY {
		ep2_new(g);
		fp2_new(a);
		fp2_new(b);
		bn_new(r);

		param = ep_param_get();

		/* I don't have a better place for this. */
		fp2_const_calc();

		switch (param) {
#if FP_PRIME == 158
			case BN_P158:
				fp_read(a[0], BN_P158_A0, strlen(BN_P158_A0), 16);
				fp_read(a[1], BN_P158_A1, strlen(BN_P158_A1), 16);
				fp_read(b[0], BN_P158_B0, strlen(BN_P158_B0), 16);
				fp_read(b[1], BN_P158_B1, strlen(BN_P158_B1), 16);
				fp_read(g->x[0], BN_P158_X0, strlen(BN_P158_X0), 16);
				fp_read(g->x[1], BN_P158_X1, strlen(BN_P158_X1), 16);
				fp_read(g->y[0], BN_P158_Y0, strlen(BN_P158_Y0), 16);
				fp_read(g->y[1], BN_P158_Y1, strlen(BN_P158_Y1), 16);
				bn_read_str(r, BN_P158_R, strlen(BN_P158_R), 16);
				break;
#elif FP_PRIME == 254
			case BN_P254:
				fp_read(a[0], BN_P254_A0, strlen(BN_P254_A0), 16);
				fp_read(a[1], BN_P254_A1, strlen(BN_P254_A1), 16);
				fp_read(b[0], BN_P254_B0, strlen(BN_P254_B0), 16);
				fp_read(b[1], BN_P254_B1, strlen(BN_P254_B1), 16);
				fp_read(g->x[0], BN_P254_X0, strlen(BN_P254_X0), 16);
				fp_read(g->x[1], BN_P254_X1, strlen(BN_P254_X1), 16);
				fp_read(g->y[0], BN_P254_Y0, strlen(BN_P254_Y0), 16);
				fp_read(g->y[1], BN_P254_Y1, strlen(BN_P254_Y1), 16);
				bn_read_str(r, BN_P254_R, strlen(BN_P254_R), 16);
				break;
#elif FP_PRIME == 256
			case BN_P256:
				fp_read(a[0], BN_P256_A0, strlen(BN_P256_A0), 16);
				fp_read(a[1], BN_P256_A1, strlen(BN_P256_A1), 16);
				fp_read(b[0], BN_P256_B0, strlen(BN_P256_B0), 16);
				fp_read(b[1], BN_P256_B1, strlen(BN_P256_B1), 16);
				fp_read(g->x[0], BN_P256_X0, strlen(BN_P256_X0), 16);
				fp_read(g->x[1], BN_P256_X1, strlen(BN_P256_X1), 16);
				fp_read(g->y[0], BN_P256_Y0, strlen(BN_P256_Y0), 16);
				fp_read(g->y[1], BN_P256_Y1, strlen(BN_P256_Y1), 16);
				bn_read_str(r, BN_P256_R, strlen(BN_P256_R), 16);
				break;
#endif
			default:
				(void)str;
				THROW(ERR_INVALID);
				break;
		}

		fp2_zero(g->z);
		fp_set_dig(g->z[0], 1);
		g->norm = 1;

		ep2_copy(&curve_g, g);
		fp_copy(curve_a[0], a[0]);
		fp_copy(curve_a[1], a[1]);
		fp_copy(curve_b[0], b[0]);
		fp_copy(curve_b[1], b[1]);
		bn_copy(&curve_r, r);

		curve_is_twist = twist;
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ep2_free(g);
		fp2_free(a);
		fp2_free(b);
		bn_free(r);
	}
}
