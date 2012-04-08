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
 * Implementation of configuration of prime elliptic curves over quadratic
 * extensions.
 *
 * @version $Id$
 * @ingroup epx
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

#if defined(EP_KBLTZ) && FP_PRIME == 158
/**
 * Parameters for a pairing-friendly prime curve over a quadratic extension.
 */
/** @{ */
#define BN_P158_A0		"0"
#define BN_P158_A1		"0"
#define BN_P158_B0		"4"
#define BN_P158_B1		"240000006ED000007FE9C000419FEC800CA035C6"
#define BN_P158_X0		"172C0A466DAFB4ACF48C9BDD0C12A435CB36CE6C"
#define BN_P158_X1		"0CE0287269D7E317EB91AF3DCD27CC373114299E"
#define BN_P158_Y0		"19A185D6B6241576480E965463B4A6A66875C184"
#define BN_P158_Y1		"074866EA7BD0AB4C67C77F70E0467F1FF32D800D"
#define BN_P158_R		"240000006ED000007FE96000419F59800C9FFD81"
/** @} */
#endif

#if defined(EP_KBLTZ) && FP_PRIME == 254
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

#if defined(EP_KBLTZ) && FP_PRIME == 256
/**
 * Parameters for a pairing-friendly prime curve over a quadratic extension.
 */
/** @{ */
#define BN_P256_A0		"0"
#define BN_P256_A1		"0"
#define BN_P256_B0		"4"
#define BN_P256_B1		"B64000000000FF2F2200000085FD5480B0001F44B6B88BF142BC818F95E3E6AE"
#define BN_P256_X0		"0C77AE4A1D6E145166739CF23DAFACA9DD396E9046424FC5479BD57692904538 "
#define BN_P256_X1		"8D1705B45D9EAAD78A9198FD8D76E2013D1BC119B4D95721A8D32F819A544F51"
#define BN_P256_Y0		"A906E963E4988478E458A4959EF7D61B570358814E28A04EF9B8C794064D73A7"
#define BN_P256_Y1		"A033144CA161E3E3271624B3F0CC1CE607ACD2CBCE9E9253C732CF3E1016DEE7"
#define BN_P256_R		"B64000000000FF2F2200000085FD547FD8001F44B6B7F4B7C2BC818F7B6BEF99"
/** @} */
#endif

#if defined(EP_KBLTZ) && FP_PRIME == 638
/**
 * Parameters for a pairing-friendly prime curve over a quadratic extension.
 */
/** @{ */
#define BN_P638_A0		"0"
#define BN_P638_A1		"0"
#define BN_P638_B0		"2"
#define BN_P638_B1		"23FFFFFDC000000D7FFFFFB8000001D3FFFFF942D000165E3FFF94870000D52FFFFDD0E00008DE55C00086520021E55BFFFFF51FFFF4EB800000004C80015ACDFFFFFFFFFFFFECE00000000000000066"
#define BN_P638_X0		"C6BA9612456EFF0E3CD291C9C1A9116FB5EEF4992E052BC5C5126F0F55F67A7D190ED74C3D6229BC3D2F645328C94554AA032352A4D7D667542F793C8FEA25AD39606CA97025AA6EF16BAC2438B1DD3"
#define BN_P638_X1		"17BE713D379D46F3D77CFF94B7226EFFB4AD01CC67A8BA712DECB3FE8FFE58A027A45523200BF6FDA534F3F59763A1F6A6461F5D2DCAC172774C0CD24BA091A37B42C6E89A1E92F3B12E3B5AFFC222BB"
#define BN_P638_Y0		"E4197B30E3A9DD98A75E6C4D2C6561B6B96083E943230D578E944E2354482212ADAAA94CA54FC4A29D6CA873EFFB27C4B61B9B822C3C217D388C6C5D04C821F1A3A8A13A37C9807323AE9CAEDD021EC"
#define BN_P638_Y1		"1A650343ACEF6895FE4EC59B49F40E043DEB05DEF170DFD71B44CAB9496E2EADD034EC0E9238544556902D2D51AB93D224DC757AD720F4DE8ED3BFA4E22DB0ECE92369F681543F23A908A9B319D5FAEF"
#define BN_P638_R		"23FFFFFDC000000D7FFFFFB8000001D3FFFFF942D000165E3FFF94870000D52FFFFDD0E00008DE55600086550021E555FFFFF54FFFF4EAC000000049800154D9FFFFFFFFFFFFEDA00000000000000061"
/** @} */
#endif

/**
 * The generator of the elliptic curve.
 */
static ep2_st curve_g;

#if ALLOC == STATIC || ALLOC == DYNAMIC || ALLOC == STACK

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

#endif

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

#if ALLOC == STACK

/**
 * In case of stack allocation, we need to get global memory for the table.
 */
static fp2_st _table[3 * EP_TABLE];

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

#if ALLOC == STATIC || ALLOC == DYNAMIC || ALLOC == STACK
	curve_g.x[0] = curve_gx[0];
	curve_g.x[1] = curve_gx[1];
	curve_g.y[0] = curve_gy[0];
	curve_g.y[1] = curve_gy[1];
	curve_g.z[0] = curve_gz[0];
	curve_g.z[1] = curve_gz[1];
#endif

#ifdef EP_PRECO
#if ALLOC == STATIC || ALLOC == DYNAMIC
	for (int i = 0; i < EP_TABLE; i++) {
		fp2_new(table[i].x);
		fp2_new(table[i].y);
		fp2_new(table[i].z);
	}
#elif ALLOC == STACK
	for (int i = 0; i < EP_TABLE; i++) {
		table[i].x[0] = _table[3 * i][0];
		table[i].x[1] = _table[3 * i][1];
		table[i].y[0] = _table[3 * i + 1][0];
		table[i].y[1] = _table[3 * i + 1][1];
		table[i].z[0] = _table[3 * i + 2][0];
		table[i].z[1] = _table[3 * i + 2][1];
	}
#endif
#endif
	ep2_set_infty(&curve_g);
	bn_init(&curve_r, FP_DIGS);
}

void ep2_curve_clean(void) {
#ifdef EP_PRECO
	for (int i = 0; i < EP_TABLE; i++) {
		fp2_free(table[i].x);
		fp2_free(table[i].y);
		fp2_free(table[i].z);
	}
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
	return (ep2_t *)*pointer;
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
#elif FP_PRIME == 638
			case BN_P638:
				fp_read(a[0], BN_P638_A0, strlen(BN_P638_A0), 16);
				fp_read(a[1], BN_P638_A1, strlen(BN_P638_A1), 16);
				fp_read(b[0], BN_P638_B0, strlen(BN_P638_B0), 16);
				fp_read(b[1], BN_P638_B1, strlen(BN_P638_B1), 16);
				fp_read(g->x[0], BN_P638_X0, strlen(BN_P638_X0), 16);
				fp_read(g->x[1], BN_P638_X1, strlen(BN_P638_X1), 16);
				fp_read(g->y[0], BN_P638_Y0, strlen(BN_P638_Y0), 16);
				fp_read(g->y[1], BN_P638_Y1, strlen(BN_P638_Y1), 16);
				bn_read_str(r, BN_P638_R, strlen(BN_P638_R), 16);
				break;
#endif
			default:
				(void)str;
				THROW(ERR_NO_VALID);
				break;
		}

		fp2_zero(g->z);
		fp_set_dig(g->z[0], 1);
		g->norm = 1;

		curve_is_twist = twist;

		ep2_copy(&curve_g, g);
		fp_copy(curve_a[0], a[0]);
		fp_copy(curve_a[1], a[1]);
		fp_copy(curve_b[0], b[0]);
		fp_copy(curve_b[1], b[1]);
		bn_copy(&curve_r, r);

		/* I don't have a better place for this. */
		fp2_const_calc();

#if defined(EP_PRECO)
		ep2_mul_pre(ep2_curve_get_tab(), &curve_g);
#endif
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
