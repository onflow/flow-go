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
 * @file
 *
 * Implementation of the prime elliptic curve utilities.
 *
 * @version $Id$
 * @ingroup ep
 */

#include <string.h>

#include "relic_core.h"
#include "relic_ep.h"
#include "relic_pp.h"
#include "relic_error.h"
#include "relic_conf.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if defined(EP_ORDIN) && FP_PRIME == 160
/**
 * Parameters for the SECG P-160 prime elliptic curve.
 */
/** @{ */
#define SECG_P160_A		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF7FFFFFFC"
#define SECG_P160_B		"1C97BEFC54BD7A8B65ACF89F81D4D4ADC565FA45"
#define SECG_P160_X		"4A96B5688EF573284664698968C38BB913CBFC82"
#define SECG_P160_Y		"23A628553168947D59DCC912042351377AC5FB32"
#define SECG_P160_R		"100000000000000000001F4C8F927AED3CA752257"
/** @} */
#endif

#if defined(EP_KBLTZ) && FP_PRIME == 160
/**
 * Parameters for the SECG K-160 prime elliptic curve.
 */
/** @{ */
#define SECG_K160_A		"0"
#define SECG_K160_B		"7"
#define SECG_K160_X		"3B4C382CE37AA192A4019E763036F4F5DD4D7EBB"
#define SECG_K160_Y		"938CF935318FDCED6BC28286531733C3F03C4FEE"
#define SECG_K160_R		"100000000000000000001B8FA16DFAB9ACA16B6B3"
#define SECG_K160_BETA	"645B7345A143464942CC46D7CF4D5D1E1E6CBB68"
#define SECG_K160_LAMB	"F3C6393C4C5C9288FE47F1DFF787A6EC6D16B2BE"
/** @} */
#endif

#if defined(EP_ORDIN) && FP_PRIME == 192
/**
 * Parameters for the NIST P-192 prime elliptic curve.
 */
/** @{ */
#define NIST_P192_A		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFFFFFFFFFFFC"
#define NIST_P192_B		"64210519E59C80E70FA7E9AB72243049FEB8DEECC146B9B1"
#define NIST_P192_X		"188DA80EB03090F67CBF20EB43A18800F4FF0AFD82FF1012"
#define NIST_P192_Y		"07192B95FFC8DA78631011ED6B24CDD573F977A11E794811"
#define NIST_P192_R		"FFFFFFFFFFFFFFFFFFFFFFFF99DEF836146BC9B1B4D22831"
/** @} */
#endif

#if defined(EP_KBLTZ) && FP_PRIME == 192
/**
 * Parameters for the SECG K-192 prime elliptic curve.
 */
/** @{ */
#define SECG_K192_A		"0"
#define SECG_K192_B		"3"
#define SECG_K192_X		"DB4FF10EC057E9AE26B07D0280B7F4341DA5D1B1EAE06C7D"
#define SECG_K192_Y		"9B2F2F6D9C5628A7844163D015BE86344082AA88D95E2F9D"
#define SECG_K192_R		"FFFFFFFFFFFFFFFFFFFFFFFE26F2FC170F69466A74DEFD8D"
#define SECG_K192_BETA	"447A96E6C647963E2F7809FEAAB46947F34B0AA3CA0BBA74"
#define SECG_K192_LAMB	"C27B0D93EDDC7284B0C2AE9813318686DBB7A0EA73692CDB"
/** @} */
#endif

#if defined(EP_ORDIN) && FP_PRIME == 224
/**
 * Parameters for the NIST P-192 prime elliptic curve.
 */
/** @{ */
#define NIST_P224_A		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFFFFFFFFFFFFFFFFFFFE"
#define NIST_P224_B		"B4050A850C04B3ABF54132565044B0B7D7BFD8BA270B39432355FFB4"
#define NIST_P224_X		"B70E0CBD6BB4BF7F321390B94A03C1D356C21122343280D6115C1D21"
#define NIST_P224_Y		"BD376388B5F723FB4C22DFE6CD4375A05A07476444D5819985007E34"
#define NIST_P224_R		"FFFFFFFFFFFFFFFFFFFFFFFFFFFF16A2E0B8F03E13DD29455C5C2A3D"
/** @} */
#endif

#if defined(EP_KBLTZ) && FP_PRIME == 224
/**
 * Parameters for the SECG K-224 prime elliptic curve.
 */
/** @{ */
#define SECG_K224_A		"0"
#define SECG_K224_B		"5"
#define SECG_K224_X		"A1455B334DF099DF30FC28A169A467E9E47075A90F7E650EB6B7A45C"
#define SECG_K224_Y		"7E089FED7FBA344282CAFBD6F7E319F7C0B0BD59E2CA4BDB556D61A5"
#define SECG_K224_R		"10000000000000000000000000001DCE8D2EC6184CAF0A971769FB1F7"
#define SECG_K224_BETA	"FE0E87005B4E83761908C5131D552A850B3F58B749C37CF5B84D6768"
#define SECG_K224_LAMB	"60DCD2104C4CBC0BE6EEEFC2BDD610739EC34E317F9B33046C9E4788"
/** @} */
#endif

#if defined(EP_ORDIN) && FP_PRIME == 256
/**
 * Parameters for the NIST P-256 prime elliptic curve.
 */
/** @{ */
#define NIST_P256_A		"FFFFFFFF00000001000000000000000000000000FFFFFFFFFFFFFFFFFFFFFFFC"
#define NIST_P256_B		"5AC635D8AA3A93E7B3EBBD55769886BC651D06B0CC53B0F63BCE3C3E27D2604B"
#define NIST_P256_X		"6B17D1F2E12C4247F8BCE6E563A440F277037D812DEB33A0F4A13945D898C296"
#define NIST_P256_Y		"4FE342E2FE1A7F9B8EE7EB4A7C0F9E162BCE33576B315ECECBB6406837BF51F5"
#define NIST_P256_R		"FFFFFFFF00000000FFFFFFFFFFFFFFFFBCE6FAADA7179E84F3B9CAC2FC632551"
/** @} */
#endif

#if defined(EP_KBLTZ) && FP_PRIME == 256
/**
 * Parameters for the SECG K-256 prime elliptic curve.
 */
/** @{ */
#define SECG_K256_A		"0"
#define SECG_K256_B		"7"
#define SECG_K256_X		"79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798"
#define SECG_K256_Y		"483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8"
#define SECG_K256_R		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141"
#define SECG_K256_BETA	"7AE96A2B657C07106E64479EAC3434E99CF0497512F58995C1396C28719501EE"
#define SECG_K256_LAMB	"5363AD4CC05C30E0A5261C028812645A122E22EA20816678DF02967C1B23BD72"
/** @} */
#endif

#if defined(EP_ORDIN) && FP_PRIME == 384
/**
 * Parameters for the NIST P-192 prime elliptic curve.
 */
/** @{ */
#define NIST_P384_A		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFFFF0000000000000000FFFFFFFC"
#define NIST_P384_B		"B3312FA7E23EE7E4988E056BE3F82D19181D9C6EFE8141120314088F5013875AC656398D8A2ED19D2A85C8EDD3EC2AEF"
#define NIST_P384_X		"AA87CA22BE8B05378EB1C71EF320AD746E1D3B628BA79B9859F741E082542A385502F25DBF55296C3A545E3872760AB7"
#define NIST_P384_Y		"3617DE4A96262C6F5D9E98BF9292DC29F8F41DBD289A147CE9DA3113B5F0B8C00A60B1CE1D7E819D7A431D7C90EA0E5F"
#define NIST_P384_R		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFC7634D81F4372DDF581A0DB248B0A77AECEC196ACCC52973"
/** @} */
#endif

#if defined(EP_ORDIN) && FP_PRIME == 521
/**
 * Parameters for the NIST P-192 prime elliptic curve.
 */
/** @{ */
#define NIST_P521_A		"1FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFC"
#define NIST_P521_B		"51953EB9618E1C9A1F929A21A0B68540EEA2DA725B99B315F3B8B489918EF109E156193951EC7E937B1652C0BD3BB1BF073573DF883D2C34F1EF451FD46B503F00"
#define NIST_P521_X		"C6858E06B70404E9CD9E3ECB662395B4429C648139053FB521F828AF606B4D3DBAA14B5E77EFE75928FE1DC127A2FFA8DE3348B3C1856A429BF97E7E31C2E5BD66"
#define NIST_P521_Y		"11839296A789A3BC0045C8A5FB42C7D1BD998F54449579B446817AFBD17273E662C97EE72995EF42640C550B9013FAD0761353C7086A272C24088BE94769FD16650"
#define NIST_P521_R		"1FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFA51868783BF2F966B7FCC0148F709A5D03BB5C9B8899C47AEBB6FB71E91386409"
/** @} */
#endif

#if defined(EP_KBLTZ) && FP_PRIME == 158
/**
 * Parameters for a 158-bit pairing-friendly prime curve.
 */
/** @{ */
#define BN_P158_A		"0"
#define BN_P158_B		"3"
#define BN_P158_X		"1"
#define BN_P158_Y		"2"
#define BN_P158_R		"240000006ED000007FE96000419F59800C9FFD81"
#define BN_P158_BETA	"240000006E8800007F434000411F93C00C7F3B08"
#define BN_P158_LAMB	"240000006E4000007E9D800040A064000C5EB323"
/** @} */
#endif

#if defined(EP_KBLTZ) && FP_PRIME == 254
/**
 * Parameters for a 254-bit pairing-friendly prime curve.
 */
/** @{ */
#define BN_P254_A		"0"
#define BN_P254_B		"2"
#define BN_P254_X		"2523648240000001BA344D80000000086121000000000013A700000000000012"
#define BN_P254_Y		"1"
#define BN_P254_R		"2523648240000001BA344D8000000007FF9F800000000010A10000000000000D"
#define BN_P254_BETA	"25236482400000017080EB4000000006181800000000000CD98000000000000B"
#define BN_P254_LAMB	"252364824000000126CD8900000000024908FFFFFFFFFFFCF9FFFFFFFFFFFFF6"
/** @} */
#endif

#if defined(EP_KBLTZ) && FP_PRIME == 256
/**
 * Parameters for a 256-bit pairing-friendly prime curve.
 */
/** @{ */
#define BN_P256_A		"0"
#define BN_P256_B		"3"
#define BN_P256_X		"1"
#define BN_P256_Y		"2"
#define BN_P256_R		"B64000000000FF2F2200000085FD547FD8001F44B6B7F4B7C2BC818F7B6BEF99"
#define BN_P256_BETA	"B64000000000ECBEAB00000073534743F00018F7D85401A5A206F14178FC918C"
#define BN_P256_LAMB	"B64000000000ECBDB800000073525A85600018F78B71E3AE0206E8EEDE64BA63"
/** @} */
#endif

/**
 * Assigns a set of ordinary elliptic curve parameters.
 *
 * @param[in] CURVE		- the curve parameters to assign.
 * @param[in] FIELD		- the finite field identifier.
 */
#define ASSIGN(CURVE, FIELD)												\
	fp_param_set(FIELD);													\
	PREPARE(str, CURVE##_A);												\
	fp_read(a, str, strlen(str), 16);										\
	PREPARE(str, CURVE##_B);												\
	fp_read(b, str, strlen(str), 16);										\
	PREPARE(str, CURVE##_X);												\
	fp_read(g->x, str, strlen(str), 16);									\
	PREPARE(str, CURVE##_Y);												\
	fp_read(g->y, str, strlen(str), 16);									\
	PREPARE(str, CURVE##_R);												\
	bn_read_str(r, str, strlen(str), 16);									\

/**
 * Assigns a set of Koblitz elliptic curve parameters.
 *
 * @param[in] CURVE		- the curve parameters to assign.
 * @param[in] FIELD		- the finite field identifier.
 */
#define ASSIGNK(CURVE, FIELD)												\
	ASSIGN(CURVE, FIELD);													\
	PREPARE(str, CURVE##_BETA);												\
	fp_read(beta, str, strlen(str), 16);									\
	PREPARE(str, CURVE##_LAMB);												\
	bn_read_str(lamb, str, strlen(str), 16);								\

/**
 * Current configured elliptic curve parameters.
 */
static int param_id;

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

int ep_param_get() {
	return param_id;
}

void ep_param_set(int param) {
	int ordin = 0;
	int kbltz = 0;
#if ARCH == AVR
	char str[2 * FP_DIGS + 1];
#else
	char *str;
#endif
	fp_t a, b, beta;
	ep_t g;
	bn_t r, lamb;

	fp_null(a);
	fp_null(b);
	fp_null(beta);
	bn_null(lamb);
	ep_null(g);
	bn_null(r);

	TRY {
		fp_new(a);
		fp_new(b);
		fp_new(beta);
		bn_new(lamb);
		ep_new(g);
		bn_new(r);

		switch (param) {
#if defined(EP_KBLTZ) && FP_PRIME == 158
			case BN_P158:
				ASSIGNK(BN_P158, BN_158);
				kbltz = 1;
				break;
#endif
#if defined(EP_ORDIN) && FP_PRIME == 160
			case SECG_P160:
				ASSIGN(SECG_P160, SECG_160);
				ordin = 1;
				break;
#endif
#if defined(EP_KBLTZ) && FP_PRIME == 160
			case SECG_K160:
				ASSIGNK(SECG_K160, SECG_160D);
				kbltz = 1;
				break;
#endif
#if defined(EP_ORDIN) && FP_PRIME == 192
			case NIST_P192:
				ASSIGN(NIST_P192, NIST_192);
				ordin = 1;
				break;
#endif
#if defined(EP_KBLTZ) && FP_PRIME == 192
			case SECG_K192:
				ASSIGNK(SECG_K192, SECG_192);
				kbltz = 1;
				break;
#endif
#if defined(EP_ORDIN) && FP_PRIME == 224
			case NIST_P224:
				ASSIGN(NIST_P224, NIST_224);
				ordin = 1;
				break;
#endif
#if defined(EP_KBLTZ) && FP_PRIME == 224
			case SECG_K224:
				ASSIGNK(SECG_K224, SECG_224);
				kbltz = 1;
				break;
#endif
#if defined(EP_KBLTZ) && FP_PRIME == 254
			case BN_P254:
				ASSIGNK(BN_P254, BN_254);
				kbltz = 1;
				break;
#endif
#if defined(EP_ORDIN) && FP_PRIME == 256
			case NIST_P256:
				ASSIGN(NIST_P256, NIST_256);
				ordin = 1;
				break;
#endif
#if defined(EP_KBLTZ) && FP_PRIME == 256
			case SECG_K256:
				ASSIGNK(SECG_K256, SECG_256);
				kbltz = 1;
				break;
			case BN_P256:
				ASSIGNK(BN_P256, BN_256);
				kbltz = 1;
				break;
#endif
#if defined(EP_ORDIN) && FP_PRIME == 384
			case NIST_P384:
				ASSIGN(NIST_P384, NIST_384);
				ordin = 1;
				break;
#endif
#if defined(EP_ORDIN) && FP_PRIME == 521
			case NIST_P521:
				ASSIGN(NIST_P521, NIST_521);
				ordin = 1;
				break;
#endif
			default:
				(void)str;
				THROW(ERR_INVALID);
				break;
		}

		param_id = param;

		fp_zero(g->z);
		fp_set_dig(g->z, 1);
		g->norm = 1;

#if defined(EP_ORDIN)
		if (ordin) {
			ep_curve_set_ordin(a, b, g, r);
		}
#endif

#if defined(EP_KBLTZ)
		if (kbltz) {
			ep_curve_set_kbltz(b, g, r, beta, lamb);
		}
#endif
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp_free(a);
		fp_free(b);
		fp_free(beta);
		bn_free(lamb);
		ep_free(g);
		bn_free(r);
	}
}

int ep_param_set_any() {
	int r0, r1;

	r0 = ep_param_set_any_ordin();
	if (r0 == STS_ERR) {
		r1 = ep_param_set_any_pairf();
		if (r1 == STS_ERR) {
			return STS_ERR;
		}
	}
	return STS_OK;
}

int ep_param_set_any_ordin() {
	int r = STS_OK;
#if FP_PRIME == 160
	ep_param_set(SECG_P160);
#elif FP_PRIME == 192
	ep_param_set(NIST_P192);
#elif FP_PRIME == 224
	ep_param_set(NIST_P224);
#elif FP_PRIME == 256
	ep_param_set(NIST_P256);
#elif FP_PRIME == 384
	ep_param_set(NIST_P384);
#elif FP_PRIME == 521
	ep_param_set(NIST_P521);
#else
	r = STS_ERR;
#endif
	return r;
}

int ep_param_set_any_kbltz() {
	int r = STS_OK;
#if FP_PRIME == 158
	ep_param_set(BN_P158);
#elif FP_PRIME == 160
	ep_param_set(SECG_K160);
#elif FP_PRIME == 192
	ep_param_set(SECG_K192);
#elif FP_PRIME == 224
	ep_param_set(SECG_K224);
#elif FP_PRIME == 254
	ep_param_set(BN_P254);
#elif FP_PRIME == 256
	ep_param_set(SECG_K256);
#else
	r = STS_ERR;
#endif
	return r;
}

int ep_param_set_any_pairf() {
	int r = STS_OK;
#if FP_PRIME == 158
	ep_param_set(BN_P158);
#elif FP_PRIME == 254
	ep_param_set(BN_P254);
#elif FP_PRIME == 256
	ep_param_set(BN_P256);
#else
	r = STS_ERR;
#endif
#ifdef WITH_PP
	ep2_curve_set(1);
#endif
	return r;
}

void ep_param_print() {
	switch (param_id) {
		case SECG_P160:
			util_print_banner("Curve SECG-P160:", 0);
			break;
		case SECG_K160:
			util_print_banner("Curve SECG-K160:", 0);
			break;
		case NIST_P192:
			util_print_banner("Curve NIST-P192:", 0);
			break;
		case SECG_K192:
			util_print_banner("Curve SECG-K192:", 0);
			break;
		case NIST_P224:
			util_print_banner("Curve NIST-P224:", 0);
			break;
		case SECG_K224:
			util_print_banner("Curve SECG-K224:", 0);
			break;
		case NIST_P256:
			util_print_banner("Curve NIST-P256:", 0);
			break;
		case SECG_K256:
			util_print_banner("Curve SECG-K256:", 0);
			break;
		case NIST_P384:
			util_print_banner("Curve NIST-P384:", 0);
			break;
		case NIST_P521:
			util_print_banner("Curve NIST-P521:", 0);
			break;
		case BN_P158:
			util_print_banner("Curve BN-P158:", 0);
			break;
		case BN_P254:
			util_print_banner("Curve BN-P254:", 0);
			break;
		case BN_P256:
			util_print_banner("Curve BN-P256:", 0);
			break;
	}
}

int ep_param_level() {
	switch (param_id) {
		case BN_P158:
			return 78;
		case SECG_P160:
		case SECG_K160:
			return 80;
		case NIST_P192:
		case SECG_K192:
			return 96;
		case NIST_P224:
		case SECG_K224:
			return 112;
		case BN_P254:
			return 126;
		case NIST_P256:
		case SECG_K256:
		case BN_P256:
			return 128;
		case NIST_P384:
			return 192;
		case NIST_P521:
			return 128;
	}
	return 0;
}
