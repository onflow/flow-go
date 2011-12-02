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
 * Implementation of the low-level binary field modular reduction functions.
 *
 * @version $Id$
 * @ingroup fb
 */

#include "relic_fb_low.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

.arch atmega128

.global fb_rdcn_low

.macro MACRO1 i
	ld		r18, -x
	mov		r23, r18
	lsr		r23
	eor		r20, r23
	std		y + (\i - 44), r20
	ldd		r20, y + (\i - 45)
	bst		r18, 0
	bld		r24, 7
	eor		r20, r24
	mov		r23, r18
	swap	r23
	andi	r23, 0xF
	eor		r19, r23
	std		y + (\i - 35), r19
	ldd		r19, y + (\i - 36)
	swap	r18
	andi	r18, 0xF0
	eor		r19, r18
.endm

.macro MACRO2 i
	ld		r18, -x
	mov		r23, r18
	lsr		r23
	eor		r20, r23
	std		y + (\i - 44), r20
	ldd		r20, y + (\i - 45)
	bst		r18, 0
	bld		r24, 7
	eor		r20, r24
	mov		r23, r18
	swap	r23
	andi	r23, 0xF
	eor		r19, r23
	std		z + (\i - 35), r19
	ldd		r19, y + (\i - 36)
	swap	r18
	andi	r18, 0xF0
	eor		r19, r18
.endm

.macro MACRO3 i
	ld		r18, -x
	mov		r23, r18
	lsr		r23
	eor		r20, r23
	std		z + (\i - 44), r20
	ldd		r20, y + (\i - 45)
	bst		r18, 0
	bld		r24, 7
	eor		r20, r24
	mov		r23, r18
	swap	r23
	andi	r23, 0xF
	eor		r19, r23
	std		z + (\i - 35), r19
	ldd		r19, y + (\i - 36)
	swap	r18
	andi	r18, 0xF0
	eor		r19, r18
.endm

fb_rdcn_low:
	push	r28
	push	r29

	movw	r30,r24				; Copy c to z
	movw	r28,r22				; Copy a to y
	movw	r26,r28				; copy a + 88 to x
	adiw	r26, 48
	adiw	r26, 41

	//c[88]
	ld		r18, -x
	ldd		r19, y + 44
	mov		r23, r18
	lsr		r23
	eor		r19, r23
	std		y + 44, r19
	ldd		r20, y + 43
	bst		r18, 0
	clr		r23
	bld		r23, 7
	eor		r20, r23
	ldd		r19, y + 53
	mov		r23, r18
	swap	r23
	andi	r23, 0xF
	eor		r19, r23
	std		y + 53, r19
	ldd		r19, y + 52
	swap	r18
	andi	r18, 0xF0
	eor		r19, r18

	clr		r24
	MACRO1 87
	MACRO1 86
	MACRO1 85
	MACRO1 84
	MACRO1 83
	MACRO1 82
	MACRO1 81
	MACRO1 80
	MACRO2 79
	MACRO2 78
	MACRO2 77
	MACRO2 76
	MACRO2 75
	MACRO2 74
	MACRO2 73
	MACRO2 72
	MACRO2 71
	MACRO2 70
	MACRO2 69
	MACRO2 68
	MACRO2 67
	MACRO2 66
	MACRO2 65
	MACRO2 64
	MACRO2 63
	MACRO2 62
	MACRO2 61
	MACRO2 60
	MACRO2 59
	MACRO2 58
	MACRO2 57
	MACRO2 56
	MACRO2 55
	MACRO2 54
	MACRO2 53
	MACRO3 52
	MACRO3 51
	MACRO3 50
	MACRO3 49
	MACRO3 48
	MACRO3 47
	MACRO3 46
	MACRO3 45

	ldd		r18, z + 44
	mov		r21, r18
	lsr		r18
	eor		r20, r18
	std		z + 0, r20
	lsl		r18

	mov		r20, r18
	swap	r20
	andi	r20, 0xF
	eor		r19, r20
	std		z + 9, r19

	mov		r22, r18
	swap	r22
	andi	r22, 0xF0
	ldd		r19, z + 8
	eor		r19, r22
	std		z + 8, r19

	eor		r21, r18
	std		z+44, r21

end:
	clr		r1

	pop		r29
	pop		r28
	ret

