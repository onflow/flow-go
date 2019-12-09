package generated

//func __converter0(p interface{}) (**int, error) {
//	var ret0 *int
//	var err error
//	if p == nil {
//		return nil, nil
//	} else {
//		// DEPTH1
//		var ret1 int
//		par1 := p.(interface{})
//		if par1 == nil {
//			ret0 = nil
//		} else {
//			ret1, err = values.CastToInt(par1)
//			if err != nil {
//				return nil, err
//			}
//
//			ret0 = &ret1
//
//		}
//		// /DEPTH1
//
//	}
//	return &ret0, nil
//
//}
//func __converter1(p interface{}) (*int, error) {
//	var ret0 int
//	var err error
//	if p == nil {
//		return nil, nil
//	} else {
//		ret0, err = values.CastToInt(p)
//		if err != nil {
//			return nil, err
//		}
//
//	}
//	return &ret0, nil
//
//}
//func __converter2(p interface{}) ([]string, error) {
//	var ret0 []string
//	var err error
//	var arr0 []interface{}
//	arr0, err = values.CastToArray(p)
//	if err != nil {
//		return nil, err
//	}
//	ret0 = make([]string, len(arr0))
//	for i0, elem0 := range arr0 {
//		ret0[i0], err = values.CastToString(elem0)
//		if err != nil {
//			return nil, err
//		}
//
//	}
//	return ret0, nil
//
//}
//func __converter3(p interface{}) ([]*string, error) {
//	var ret0 []*string
//	var err error
//	var arr0 []interface{}
//	arr0, err = values.CastToArray(p)
//	if err != nil {
//		return nil, err
//	}
//	ret0 = make([]*string, len(arr0))
//	for i0, elem0 := range arr0 {
//		// DEPTH1
//		var ret1 string
//		par1 := elem0.(interface{})
//		if par1 == nil {
//			ret0[i0] = nil
//		} else {
//			ret1, err = values.CastToString(par1)
//			if err != nil {
//				return nil, err
//			}
//
//			ret0[i0] = &ret1
//
//		}
//		// /DEPTH1
//
//	}
//	return ret0, nil
//
//}
