package handlers

import (
	"fmt"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	zhTranslations "github.com/go-playground/validator/v10/translations/zh"
)

var (
	validate   *validator.Validate
	translator ut.Translator
)

func init() {
	validate = validator.New()

	zhTranslator := zh.New()
	uni := ut.New(zhTranslator, zhTranslator)
	translator, _ = uni.GetTranslator("zh")
	_ = zhTranslations.RegisterDefaultTranslations(validate, translator)

	// 注册自定义标签翻译
	//_ = validate.RegisterTranslation("required", translator, func(ut ut.Translator) error {
	//	return ut.Add("required", "{0}不能为空", true)
	//}, func(ut ut.Translator, fe validator.FieldError) string {
	//	t, _ := ut.T("required", fe.Field())
	//	return t
	//})
}

// validateStruct 用于验证结构体
func validateStruct(s any) error {
	if err := validate.Struct(s); err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			return err
		}

		// 翻译错误信息
		var errMsgs []string
		for _, err := range err.(validator.ValidationErrors) {
			errMsgs = append(errMsgs, err.Translate(translator))
		}
		return fmt.Errorf("参数验证失败：%v", errMsgs)
	}
	return nil
}
