package com.github.zhoukq.k3s.food.remote;

import com.github.zhoukq.k3s.food.entiy.Product;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.InitializingBean;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.core.ParameterizedTypeReference;
import org.springframework.http.client.ClientHttpRequestFactory;
import org.springframework.stereotype.Component;
import org.springframework.web.client.RestClient;

import java.util.List;

@Component
public class DigitalRemote implements InitializingBean {

    private RestClient restClient;
    @Autowired
    private ClientHttpRequestFactory clientHttpRequestFactory;

    @Value("${remote.digitalUrl}")
    private String digitalUrl;
    public static final Logger LOGGER = LoggerFactory.getLogger(DigitalRemote.class);

    public List<Product> getProductList() {
        try {
            return restClient.get()
                    .uri("/products")
                    .retrieve()
                    .body(new ParameterizedTypeReference<>() {
                    });
        } catch (Exception e) {
            LOGGER.error(e.getLocalizedMessage());
            return List.of();
        }
    }

    @Override
    public void afterPropertiesSet() throws Exception {
        this.restClient = RestClient.builder().requestFactory(clientHttpRequestFactory).baseUrl(digitalUrl).build();
    }
}
