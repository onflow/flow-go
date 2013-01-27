/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2013 RELIC Authors
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
 * Implementation of the low-level binary field square root.
 *
 * @version $Id$
 * @ingroup fb
 */

#include "relic_fb_low.h"

.arch atmega128

.text

.global fb_srtp_low

.macro MACRO
	ld		r18, x+
	bst		r18, 0
	bld		r19, 0
	bst		r18, 2
	bld		r19, 1
	bst		r18, 4
	bld		r19, 2
	bst		r18, 6
	bld		r19, 3

	bst		r18, 1
	bld		r20, 0
	bst		r18, 3
	bld		r20, 1
	bst		r18, 5
	bld		r20, 2
	bst		r18, 7
	bld		r20, 3

	ld		r18, x+
	bst		r18, 0
	bld		r19, 4
	bst		r18, 2
	bld		r19, 5
	bst		r18, 4
	bld		r19, 6
	bst		r18, 6
	bld		r19, 7

	bst		r18, 1
	bld		r20, 4
	bst		r18, 3
	bld		r20, 5
	bst		r18, 5
	bld		r20, 6
	bst		r18, 7
	bld		r20, 7
.endm

fb_srtp_low:
	push r28
	push r29

	movw	r30,r24				; Copy c to Z
	movw	r28,r22				; Copy t to Y
	movw	r26,r20				; Copy a to X

	MACRO
	std		z+0, r19
	std		y+17,r20
	std		y+13, r20
	std		y+11, r20
	std		y+7, r20

	MACRO
	std		z+1, r19
	std		y+18,r20
	std		y+14, r20
	std		y+12, r20
	std		y+8, r20

	MACRO
	std		z+2, r19
	std		y+19,r20
	std		y+15, r20
	ldd		r21, y+13
	eor		r21, r20
	std		y+13, r21
	std		y+9, r20

	MACRO
	std		z+3, r19
	std		y+20,r20
	std		y+16, r20
	ldd		r21, y+14
	eor		r21, r20
	std		y+14, r21
	std		y+10, r20

	MACRO
	std		z+4, r19
	std		y+21,r20
	ldd		r21, y+17
	eor		r21, r20
	std		y+17, r21
	ldd		r22, y+15
	eor		r22, r20
	std		y+15, r22
	ldd		r23, y+11
	eor		r23, r20
	std		y+11, r23

	MACRO
	std		z+5, r19
	std		y+22,r20
	ldd		r21, y+18
	eor		r21, r20
	std		y+18, r21
	ldd		r22, y+16
	eor		r22, r20
	std		y+16, r22
	ldd		r23, y+12
	eor		r23, r20
	std		y+12, r23

	MACRO
	std		z+6, r19
	std		y+23,r20
	ldd		r21, y+19
	eor		r21, r20
	std		y+19, r21
	ldd		r22, y+17
	eor		r22, r20
	std		y+17, r22
	ldd		r23, y+13
	eor		r23, r20
	std		y+13, r23

	MACRO
	ldd		r24, y+7
	eor		r24, r19
	std		z+7, r24
	std		y+24,r20
	ldd		r21, y+20
	eor		r21, r20
	std		y+20, r21
	ldd		r22, y+18
	eor		r22, r20
	std		y+18, r22
	ldd		r23, y+14
	eor		r23, r20
	std		y+14, r23

	MACRO
	ldd		r24, y+8
	eor		r24, r19
	std		z+8, r24
	std		y+25,r20
	ldd		r21, y+21
	eor		r21, r20
	std		y+21, r21
	ldd		r22, y+19
	eor		r22, r20
	std		y+19, r22
	ldd		r23, y+15
	eor		r23, r20
	std		y+15, r23

	MACRO
	ldd		r24, y+9
	eor		r24, r19
	std		z+9, r24
	std		y+26,r20
	ldd		r21, y+22
	eor		r21, r20
	std		y+22, r21
	ldd		r22, y+20
	eor		r22, r20
	std		y+20, r22
	ldd		r23, y+16
	eor		r23, r20
	std		y+16, r23

	MACRO
	ldd		r24, y+10
	eor		r24, r19
	std		z+10, r24
	std		y+27,r20
	ldd		r21, y+23
	eor		r21, r20
	std		y+23, r21
	ldd		r22, y+21
	eor		r22, r20
	std		y+21, r22
	ldd		r23, y+17
	eor		r23, r20
	std		z+17, r23

	MACRO
	ldd		r24, y+11
	eor		r24, r19
	std		z+11, r24
	std		y+28,r20
	ldd		r21, y+24
	eor		r21, r20
	std		y+24, r21
	ldd		r22, y+22
	eor		r22, r20
	std		y+22, r22
	ldd		r23, y+18
	eor		r23, r20
	std		z+18, r23

	MACRO
	ldd		r24, y+12
	eor		r24, r19
	std		z+12, r24
	std		y+29,r20
	ldd		r21, y+25
	eor		r21, r20
	std		y+25, r21
	ldd		r22, y+23
	eor		r22, r20
	std		y+23, r22
	ldd		r23, y+19
	eor		r23, r20
	std		z+19, r23

	MACRO
	ldd		r24, y+13
	eor		r24, r19
	std		z+13, r24
	std		y+30,r20
	ldd		r21, y+26
	eor		r21, r20
	std		y+26, r21
	ldd		r22, y+24
	eor		r22, r20
	std		z+24, r22
	ldd		r23, y+20
	eor		r23, r20
	std		z+20, r23

	MACRO
	ldd		r24, y+14
	eor		r24, r19
	std		z+14, r24
	std		y+31,r20
	ldd		r21, y+27
	eor		r21, r20
	std		y+27, r21
	ldd		r22, y+25
	eor		r22, r20
	std		z+25, r22
	ldd		r23, y+21
	eor		r23, r20
	std		z+21, r23

	MACRO
	ldd		r24, y+15
	eor		r24, r19
	std		z+15, r24
	std		y+32,r20
	ldd		r21, y+28
	eor		r21, r20
	std		z+28, r21
	ldd		r22, y+26
	eor		r22, r20
	std		z+26, r22
	ldd		r23, y+22
	eor		r23, r20
	std		z+22, r23

	MACRO
	ldd		r24, y+16
	eor		r24, r19
	std		z+16, r24
	std		z+33, r20
	ldd		r21, y+29
	eor		r21, r20
	std		z+29, r21
	ldd		r22, y+27
	eor		r22, r20
	std		z+27, r22
	ldd		r23, y+23
	eor		r23, r20
	std		z+23, r23

	ldd r19, y+30
	std z+30, r19
	ldd r19, y+31
	std z+31, r19
	ldd r19, y+32
	std z+32, r19

	pop r29
	pop r28
	ret
