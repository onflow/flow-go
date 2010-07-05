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
 * Implementation of the low-level ternary field cubing.
 *
 * @version $Id$
 * @ingroup ft
 */

#include "relic_ft.h"
#include "relic_dv.h"
#include "relic_ft_low.h"
#include "relic_util.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if WORD > 8
/**
 * Precomputed table of the cubing of all polynomial with degree less than 4.
 */
static const dig_t table[16] = {
	0x000, 0x001, 0x008, 0x009,
	0x040, 0x041, 0x048, 0x049,
	0x200, 0x201, 0x208, 0x209,
	0x240, 0x241, 0x248, 0x249,
};
#else
static const dig_t table_l[16] = {
	0x00, 0x01, 0x08, 0x09,
	0x40, 0x41, 0x48, 0x49,
	0x00, 0x01, 0x08, 0x09,
	0x40, 0x41, 0x48, 0x49,
};

static const dig_t table_m[16] = {
	0x00, 0x00, 0x00, 0x00,
	0x04, 0x04, 0x04, 0x04,
	0x20, 0x20, 0x20, 0x20,
	0x24, 0x24, 0x24, 0x24,
};

static const dig_t table_h[16] = {
	0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00,
	0x02, 0x02, 0x02, 0x02,
	0x02, 0x02, 0x02, 0x02,
};
#endif

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void ft_cubn_low(dig_t *c, dig_t *a) {
	int i, j, d, trit;

	dv_zero(c, 3 * FT_DIGS);
	for (i = 0; i < FT_TRITS; i++) {
		j = ft_get_trit(a, i);
		SPLIT(trit, d, 3 * i, FT_DIG_LOG);
		switch (j % 3) {
			case 0:
				ft_set_bit(c + d, trit, 0);
				ft_set_bit(c + 3 * FT_DIGS / 2 + d, trit, 0);
				break;
			case 1:
				ft_set_bit(c + d, trit, 1);
				ft_set_bit(c + 3 * FT_DIGS / 2 + d, trit, 0);
				break;
			case 2:
				ft_set_bit(c + d, trit, 0);
				ft_set_bit(c + 3 * FT_DIGS / 2 + d, trit, 1);
				break;
		}
	}
}

void ft_cubl_low(dig_t *c, dig_t *a) {
	dig_t d, *tmpt;

	tmpt = c;
#if WORD == 8
	for (int i = 0; i < FT_DIGS; i++, tmpt++) {
		d = a[i];
		*tmpt = table_l[LOW(d)];
		tmpt++;
		*tmpt = (table_h[LOW(d)]) | ((table_l[HIGH(d)] << 4) & 0xF0);
		tmpt++;
		*tmpt = table_m[HIGH(d)];
	}
#elif WORD == 16
	for (int i = 0; i < FT_DIGS; i++, tmpt++) {
		d = a[i];
		*tmpt = (table[d & 0xF]) | (table[(d >> 4) & 0xF] << 12);
		tmpt++;
		*tmpt = (table[(d >> 4) & 0xF] >> 4) | (table[(d >> 8) & 0xF] << 8);
		tmpt++;
		*tmpt = (table[(d >> 8) & 0xF] >> 8) | (table[(d >> 12) & 0xF] << 4);
	}
#elif WORD == 32
	for (int i = 0; i < FT_DIGS; i++, tmpt++) {
		d = a[i];
		*tmpt = (table[d & 0xF]) | (table[(d >> 4) & 0xF] << 12) | (table[(d >> 8) & 0xF] << 24);
		tmpt++;
		*tmpt = (table[(d >> 8) & 0xF] >> 8) | (table[(d >> 12) & 0xF] << 4) | (table[(d >> 16) & 0xF] << 16) | (table[(d >> 20) & 0xF] << 28);
		*tmpt++;
		*tmpt = (table[(d >> 20) & 0xF] >> 4) | (table[(d >> 24) & 0xF] << 8) | (table[(d >> 28) & 0xF] << 20);
	}
#elif WORD == 64
	for (int i = 0; i < FT_DIGS; i++, tmpt++) {
		d = a[i];
		*tmpt = (table[d & 0xF] << 00) | (table[(d >> 4) & 0xF] << 12) |
				(table[(d >> 8) & 0xF] << 24) | (table[(d >> 12) & 0xF] << 36) |
				(table[(d >> 16) & 0xF] << 48) | (table[(d >> 20) & 0xF] << 60);
		tmpt++;
		*tmpt = (table[(d >> 20) & 0xF] >> 4) | (table[(d >> 24) & 0xF] << 8) |
				(table[(d >> 28) & 0xF] << 20) | (table[(d >> 32) & 0xF] << 32) |
				(table[(d >> 36) & 0xF] << 44) | (table[(d >> 40) & 0xF] << 56);
		tmpt++;
		*tmpt =	(table[(d >> 40) & 0xF] >> 8) | (table[(d >> 44) & 0xF] << 4) |
				(table[(d >> 48) & 0xF] << 16) | (table[(d >> 52) & 0xF] << 28) |
				(table[(d >> 56) & 0xF] << 40) | (table[(d >> 60) & 0xF] << 52);
	}
#endif
}

void ft_cubm_low(dig_t *c, dig_t *a) {
	dig_t align t[3 * FT_DIGS];

	ft_cubl_low(t, a);
	ft_rdcc_low(c, t);
}
