package com.github.zhoukq.k3s.food.controller;

import com.github.zhoukq.k3s.food.entiy.Product;
import com.github.zhoukq.k3s.food.remote.DigitalRemote;
import com.github.zhoukq.k3s.food.remote.FoodRemote;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Controller;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.servlet.ModelAndView;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

@Controller
@RequestMapping("/")
public class HomeController {
    @Autowired
    private DigitalRemote digitalRemote;
    @Autowired
    private FoodRemote foodRemote;

    public static final Logger LOGGER = LoggerFactory.getLogger(HomeController.class);

    @RequestMapping("/")
    public ModelAndView welcome() {
        ModelAndView modelAndView = new ModelAndView("index");
        modelAndView.addObject("message", "Welcome to supermarket!");
        return modelAndView;
    }

    @RequestMapping("/shopping")
    public ModelAndView shopping() {
        ModelAndView modelAndView = new ModelAndView("shopping");
        Map<String, List<Product>> productMap = loadProductMap();
        modelAndView.addObject("productMap", productMap);
        return modelAndView;
    }

    private Map<String, List<Product>> loadProductMap() {
        LinkedHashMap<String, List<Product>> map = new LinkedHashMap<>();
        map.put("手机数码", digitalRemote.getProductList());
        map.put("零食饮料", foodRemote.getProductList());
        return map;
    }


}
